import { useState } from 'react';
import type { FormEvent } from 'react';
import { AssistantRuntimeProvider, MessagePrimitive, ThreadPrimitive, useExternalStoreRuntime, useMessage, type AppendMessage } from '@assistant-ui/react';
import { Copy, Loader2, Send, Trash2 } from 'lucide-react';
import { WhatsAppIcon } from './wa-brand-icon';
import { toAssistantMessage, type WaChatEvent, type WaChatMeta, type WaContact } from './wa-chat-model';
import { WaMessageContent } from './wa-message-content';
import { Badge, Button, Input } from './ui';

export function WaChatThread({ contact, events, loading, sending, error, onSendMessage, onDeleteMessage }: { contact?: WaContact; events: WaChatEvent[]; loading: boolean; sending: boolean; error?: string; onSendMessage: (text: string) => Promise<unknown>; onDeleteMessage: (messageID: string) => void }) {
  const runtime = useExternalStoreRuntime<WaChatEvent>({ messages: events, convertMessage: toAssistantMessage, isDisabled: true, isLoading: loading, onNew: noopNewMessage });
  const title = contact?.title || '选择联系人';
  return (
    <section className="grid min-h-0 grid-rows-[auto_1fr_auto] overflow-hidden bg-card">
      <ChatHeader contact={contact} loading={loading} />
      <div className="h-full min-h-0">
        <AssistantRuntimeProvider runtime={runtime}>
          <ThreadPrimitive.Root className="h-full min-h-0">
            <ThreadPrimitive.Viewport autoScroll className="h-full min-h-0 space-y-3 overflow-y-auto bg-[#f6f8fb] p-5">
              <ThreadPrimitive.Empty><EmptyConversation title={title} /></ThreadPrimitive.Empty>
              <ThreadPrimitive.Messages>{() => <BubbleMessage onDeleteMessage={onDeleteMessage} />}</ThreadPrimitive.Messages>
            </ThreadPrimitive.Viewport>
          </ThreadPrimitive.Root>
        </AssistantRuntimeProvider>
      </div>
      <ChatComposer disabled={!contact || sending} error={error} onSendMessage={onSendMessage} />
    </section>
  );
}

function ChatHeader({ contact, loading }: { contact?: WaContact; loading: boolean }) {
  const subtitle = contact?.subtitle || '请选择左侧联系人';
  return (
    <header className="flex h-16 items-center justify-between gap-3 border-b border-border px-5">
      <div className="flex min-w-0 items-center gap-3">
        <span className="grid size-10 place-items-center rounded-full bg-emerald-50"><WhatsAppIcon className="size-7" /></span>
        <div className="min-w-0">
          <h2 className="truncate text-sm font-semibold">{contact?.title || '暂无联系人'}</h2>
          <p className="truncate text-xs text-muted-foreground">{subtitle}</p>
        </div>
      </div>
      <div className="flex items-center gap-2">
        {loading && <Loader2 className="size-4 animate-spin text-muted-foreground" />}
      </div>
    </header>
  );
}

function BubbleMessage({ onDeleteMessage }: { onDeleteMessage: (messageID: string) => void }) {
  const meta = useMessage((message) => message.metadata.custom as WaChatMeta | undefined);
  const outgoing = Boolean(meta?.outgoing);
  const unread = Boolean(meta?.canMarkRead && !meta.read);
  const messageID = useMessage((message) => message.id);
  return (
    <MessagePrimitive.Root className={`flex w-full ${outgoing ? 'justify-end' : 'justify-start'}`}>
      <div className={`max-w-[min(640px,82%)] rounded-3xl border px-4 py-3 shadow-sm ${outgoing ? 'rounded-tr-md border-emerald-200 bg-emerald-50' : unread ? 'rounded-tl-md border-emerald-200 bg-emerald-50/70' : 'rounded-tl-md border-border bg-card'}`}>
        <div className="mb-1 flex items-center gap-2 text-[11px] text-muted-foreground"><span>{meta?.source || '消息'}</span>{unread && <Badge>未读</Badge>}<span>·</span><MessageTime /></div>
        <div className="flex items-start gap-3">
          <WaMessageContent text={meta?.displayText || ''} />
          {meta?.copyText && <CopyButton text={meta.copyText} />}
          <DeleteButton messageID={messageID} onDeleteMessage={onDeleteMessage} />
        </div>
      </div>
    </MessagePrimitive.Root>
  );
}

function MessageTime() {
  const createdAt = useMessage((message) => message.createdAt);
  return createdAt ? <time>{createdAt.toLocaleString()}</time> : null;
}

function CopyButton({ text }: { text: string }) {
  return <Button className="rounded-full text-muted-foreground hover:text-foreground" variant="ghost" size="icon-sm" type="button" title="复制" aria-label="复制" onClick={() => void navigator.clipboard?.writeText(text)}><Copy size={14} /></Button>;
}

function DeleteButton({ messageID, onDeleteMessage }: { messageID: string; onDeleteMessage: (messageID: string) => void }) {
  return <Button className="rounded-full text-muted-foreground hover:bg-destructive/10 hover:text-destructive" variant="ghost" size="icon-sm" type="button" title="删除" aria-label="删除" onClick={() => onDeleteMessage(messageID)}><Trash2 size={14} /></Button>;
}

function ChatComposer({ disabled, error, onSendMessage }: { disabled: boolean; error?: string; onSendMessage: (text: string) => Promise<unknown> }) {
  const [text, setText] = useState('');
  const trimmed = text.trim();
  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!trimmed || disabled) return;
    try {
      await onSendMessage(trimmed);
      setText('');
    } catch {
      // React Query surfaces the error in the thread footer.
    }
  }
  return (
    <footer className="border-t border-border px-5 py-3">
      <form className="flex items-center gap-2" onSubmit={(event) => void submit(event)}>
        <Input value={text} onChange={(event) => setText(event.target.value)} disabled={disabled} placeholder={disabled ? '选择联系人后发送' : '输入消息'} aria-label="消息内容" autoComplete="off" />
        <Button size="icon" type="submit" disabled={disabled || !trimmed} title="发送" aria-label="发送"><Send size={16} /></Button>
      </form>
      <p className={`mt-2 text-xs ${error ? 'text-destructive' : 'text-muted-foreground'}`}>{error || '仅支持已有 Signal 会话的 1:1 文本消息。'}</p>
    </footer>
  );
}

function EmptyConversation({ title }: { title: string }) {
  return <div className="mx-auto mt-16 max-w-sm rounded-2xl bg-card/90 p-6 text-center text-sm text-muted-foreground shadow-sm"><WhatsAppIcon className="mx-auto mb-3 size-9" /><p className="font-medium text-foreground">{title}</p><p className="mt-1">选择联系人或等待新消息。</p></div>;
}

async function noopNewMessage(_message: AppendMessage) {}
