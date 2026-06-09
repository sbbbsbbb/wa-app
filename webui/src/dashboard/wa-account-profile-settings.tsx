import { type FormEvent, useEffect, useRef, useState } from 'react';
import { Check, ImagePlus, Loader2, Trash2, UserRound, X } from 'lucide-react';
import { useMutation } from '@tanstack/react-query';
import type { WAAccount } from '../proto/byte/v/forge/waapp/v1/profile';
import { removeWaAccountProfilePicture, setWaAccountProfileName, setWaAccountProfilePicture } from './wa-api';
import { WhatsAppIcon } from './wa-brand-icon';
import { Badge, Button, Field, FieldDescription, FieldGroup, FieldLabel, Input } from './ui';

const maxProfilePictureBytes = 2 * 1024 * 1024;

type Props = {
  account: WAAccount;
  onDone: (message: string) => void;
  onError: (message: string) => void;
};

export function WaAccountProfileSettings({ account, onDone, onError }: Props) {
  const [displayName, setDisplayName] = useState('');
  const [picture, setPicture] = useState<File | null>(null);
  const [lastPictureID, setLastPictureID] = useState('');
  const fileInput = useRef<HTMLInputElement>(null);
  const preview = usePicturePreview(picture);
  const handleError = (error: unknown) => onError(error instanceof Error ? error.message : String(error));
  const nameMutation = useMutation({
    mutationFn: () => setWaAccountProfileName(account, displayName),
    onSuccess: (resp) => { setDisplayName(''); onDone(statusDoneMessage('资料名称请求已提交', resp.operation?.status)); },
    onError: handleError,
  });
  const pictureMutation = useMutation({
    mutationFn: async () => {
      if (!picture) throw new Error('请选择头像图片');
      if (picture.size > maxProfilePictureBytes) throw new Error('头像图片不能超过 2 MiB');
      return setWaAccountProfilePicture(account, { image_base64: await fileBase64(picture), content_type: picture.type || 'application/octet-stream' });
    },
    onSuccess: (resp) => {
      setPicture(null);
      if (fileInput.current) fileInput.current.value = '';
      setLastPictureID(resp.profile_picture_id || '');
      onDone(resp.profile_picture_id ? '头像已提交' : '头像请求已提交');
    },
    onError: handleError,
  });
  const removeMutation = useMutation({
    mutationFn: () => removeWaAccountProfilePicture(account),
    onSuccess: () => { setPicture(null); setLastPictureID(''); if (fileInput.current) fileInput.current.value = ''; onDone('头像移除请求已提交'); },
    onError: handleError,
  });
  const busy = nameMutation.isPending || pictureMutation.isPending || removeMutation.isPending;
  return (
    <section className="grid gap-4 rounded-xl border border-border bg-card p-4">
      <div className="flex items-start justify-between gap-3">
        <div className="grid gap-1"><h3 className="inline-flex items-center gap-2 text-sm font-semibold"><UserRound size={15} />资料设置</h3><p className="text-xs text-muted-foreground">设置当前 WA 账号头像和显示名称。</p></div>
        <Badge variant="outline">{busy ? '提交中' : '就绪'}</Badge>
      </div>
      <div className="grid gap-4 lg:grid-cols-[11rem_minmax(0,1fr)]">
        <form className="grid gap-3 rounded-2xl border border-border bg-background p-3" onSubmit={(event) => submit(event, pictureMutation.mutate)}>
          <div className="mx-auto grid size-28 place-items-center overflow-hidden rounded-full bg-emerald-50 ring-1 ring-border/60">{preview ? <img className="size-full object-cover" src={preview} alt="待提交头像预览" /> : <WhatsAppIcon className="size-16" />}</div>
          <Input ref={fileInput} className="hidden" type="file" accept="image/jpeg,image/png,image/webp" disabled={busy} onChange={(event) => setPicture(event.target.files?.[0] || null)} />
          <div className="flex justify-center gap-2">
            <Button type="button" size="sm" variant="outline" disabled={busy} title="选择头像" aria-label="选择头像" onClick={() => fileInput.current?.click()}><ImagePlus size={15} /></Button>
            <Button type="submit" size="sm" disabled={busy || !picture} title="提交头像" aria-label="提交头像">{pictureMutation.isPending ? <Loader2 className="size-4 animate-spin" /> : <Check size={15} />}</Button>
            <Button type="button" size="sm" variant="outline" disabled={busy || !picture} title="取消选择" aria-label="取消选择" onClick={() => { setPicture(null); if (fileInput.current) fileInput.current.value = ''; }}><X size={15} /></Button>
            <Button type="button" size="sm" variant="destructive" disabled={busy} title="移除头像" aria-label="移除头像" onClick={() => removeMutation.mutate()}><Trash2 size={15} /></Button>
          </div>
          <p className="truncate text-center text-xs text-muted-foreground">{picture ? `${picture.name} · ${formatBytes(picture.size)}` : lastPictureID ? `已提交 ID ${lastPictureID}` : 'JPEG / PNG / WebP，最大 2 MiB'}</p>
        </form>
        <form className="rounded-2xl border border-border bg-background p-3" onSubmit={(event) => submit(event, nameMutation.mutate)}>
          <FieldGroup>
            <Field>
              <FieldLabel>显示名称</FieldLabel>
              <div className="grid gap-2 sm:grid-cols-[minmax(0,1fr)_auto]">
                <Input value={displayName} onChange={(event) => setDisplayName(event.target.value)} maxLength={25} disabled={busy} placeholder="输入 WA 资料名称" />
                <Button type="submit" disabled={busy || !displayName.trim()} title="提交名称" aria-label="提交名称">{nameMutation.isPending ? <Loader2 className="size-4 animate-spin" /> : <Check size={15} />}</Button>
              </div>
            </Field>
            <FieldDescription>最多 25 个字符；服务端会按 WA app-state 名称接口提交。</FieldDescription>
          </FieldGroup>
        </form>
      </div>
    </section>
  );
}

function submit(event: FormEvent<HTMLFormElement>, run: () => void) {
  event.preventDefault();
  run();
}

async function fileBase64(file: File) {
  const bytes = new Uint8Array(await file.arrayBuffer());
  let binary = '';
  for (let index = 0; index < bytes.length; index += 0x8000) {
    binary += String.fromCharCode(...bytes.subarray(index, index + 0x8000));
  }
  return btoa(binary);
}

function formatBytes(value: number) {
  if (value < 1024) return `${value} B`;
  if (value < 1024 * 1024) return `${(value / 1024).toFixed(1)} KiB`;
  return `${(value / 1024 / 1024).toFixed(1)} MiB`;
}

function statusDoneMessage(message: string, status?: unknown) {
  return status ? `${message}：${String(status).replace('ACCOUNT_SETTINGS_OPERATION_STATUS_', '')}` : message;
}

function usePicturePreview(file: File | null) {
  const [preview, setPreview] = useState('');
  useEffect(() => {
    if (!file) {
      setPreview('');
      return undefined;
    }
    const objectURL = URL.createObjectURL(file);
    setPreview(objectURL);
    return () => URL.revokeObjectURL(objectURL);
  }, [file]);
  return preview;
}
