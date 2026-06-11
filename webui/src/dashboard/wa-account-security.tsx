import { type FormEvent, useState } from 'react';
import { CheckCircle2, KeyRound, Mail, Send, ShieldCheck } from 'lucide-react';
import { useMutation, useQuery } from '@tanstack/react-query';
import { AccountSettingsOperationStatus } from '../proto/byte/v/forge/waapp/v1/account_settings';
import type { WaAccountProjection } from './wa-api';
import { getWaTwoFactorAuthStatus, requestWaAccountEmailOtp, setWaAccountEmail, setWaTwoFactorAuthSettings, verifyWaAccountEmailOtp, waAccountID, waKeys } from './wa-api';
import { Badge, type BadgeVariant, Button, Field, FieldGroup, FieldLabel, Input } from './ui';

type Props = { account: WaAccountProjection; onDone: (message: string) => void; onError: (message: string) => void };

export function WaAccountSecurityPanel({ account, onDone, onError }: Props) {
  const [pin, setPin] = useState('');
  const [email, setEmail] = useState('');
  const [emailOtp, setEmailOtp] = useState('');
  const [emailOtpVisible, setEmailOtpVisible] = useState(false);
  const [lastStatus, setLastStatus] = useState<AccountSettingsOperationStatus | undefined>();
  const handleError = (error: unknown) => onError(error instanceof Error ? error.message : String(error));
  const handleSuccess = (message: string, status?: AccountSettingsOperationStatus) => { setLastStatus(status); onDone(message); };
  const twoFactorStatus = useQuery({
    queryKey: waKeys.twoFactorStatus(waAccountID(account)),
    queryFn: () => getWaTwoFactorAuthStatus(account),
    enabled: Boolean(waAccountID(account)),
    staleTime: 30_000,
  });
  const twoFactor = useMutation({
    mutationFn: () => setWaTwoFactorAuthSettings(account, pin),
    onSuccess: (resp) => {
      setPin('');
      void twoFactorStatus.refetch();
      handleSuccess('2FA PIN 设置请求已提交', resp.operation?.status);
    },
    onError: handleError,
  });
  const emailSet = useMutation({
    mutationFn: () => setWaAccountEmail(account, { email_address: email }),
    onSuccess: (resp) => {
      const status = resp.operation?.status;
      setEmailOtpVisible(shouldShowEmailOtp(status));
      if (status === AccountSettingsOperationStatus.ACCOUNT_SETTINGS_OPERATION_STATUS_VERIFIED) setEmailOtp('');
      void twoFactorStatus.refetch();
      handleSuccess('账户邮箱设置请求已提交', status);
    },
    onError: handleError,
  });
  const otpRequest = useMutation({
    mutationFn: () => requestWaAccountEmailOtp(account),
    onSuccess: (resp) => {
      setEmailOtpVisible(true);
      handleSuccess('邮箱 OTP 已请求', resp.operation?.status);
    },
    onError: handleError,
  });
  const otpVerify = useMutation({
    mutationFn: () => verifyWaAccountEmailOtp(account, emailOtp),
    onSuccess: (resp) => {
      const status = resp.operation?.status;
      setEmailOtp('');
      setEmailOtpVisible(shouldShowEmailOtp(status));
      void twoFactorStatus.refetch();
      handleSuccess('邮箱 OTP 校验请求已提交', status);
    },
    onError: handleError,
  });
  const busy = twoFactor.isPending || emailSet.isPending || otpRequest.isPending || otpVerify.isPending;
  const handleEmailChange = (value: string) => { setEmail(value); setEmailOtp(''); setEmailOtpVisible(false); };
  return (
    <section className="grid gap-5">
      <div className="flex items-center justify-end"><Badge variant="outline">{statusLabel(lastStatus)}</Badge></div>
      <div className="grid gap-6 lg:grid-cols-2">
        <form className="grid gap-3" onSubmit={(event) => submit(event, twoFactor.mutate)}>
          <div className="inline-flex items-center gap-2 text-sm font-medium">
            <ShieldCheck size={15} />设置 2FA PIN
            <Badge variant={twoFactorBadgeVariant(twoFactorStatus)}>{twoFactorStatusLabel(twoFactorStatus)}</Badge>
          </div>
          <FieldGroup><Field><FieldLabel>6 位 PIN</FieldLabel><Input value={pin} onChange={(event) => setPin(event.target.value)} inputMode="numeric" autoComplete="one-time-code" type="password" maxLength={6} disabled={busy} /></Field><Button type="submit" disabled={busy || pin.length !== 6}><KeyRound size={14} />提交 PIN</Button></FieldGroup>
        </form>
        <form className="grid gap-3" onSubmit={(event) => submit(event, emailSet.mutate)}>
          <div className="inline-flex items-center gap-2 text-sm font-medium"><Mail size={15} />设置账户邮箱</div>
          <FieldGroup>
            <Field><FieldLabel>邮箱地址</FieldLabel><Input value={email} onChange={(event) => handleEmailChange(event.target.value)} type="email" disabled={busy} /></Field>
            <Button type="submit" disabled={busy || !email}><Mail size={14} />提交邮箱</Button>
          </FieldGroup>
        </form>
        {emailOtpVisible && (
          <div className="grid gap-3 border-t border-border pt-5 lg:col-span-2">
            <div className="flex items-center gap-2 text-sm font-medium"><Send size={15} />邮箱 OTP</div>
            <div className="grid gap-3 sm:grid-cols-[auto_1fr_auto]">
              <Button type="button" variant="outline" disabled={busy} onClick={() => otpRequest.mutate()}><Send size={14} />请求 OTP</Button>
              <Input value={emailOtp} onChange={(event) => setEmailOtp(event.target.value)} inputMode="numeric" autoComplete="one-time-code" type="password" maxLength={6} disabled={busy} placeholder="6 位验证码" />
              <Button type="button" disabled={busy || emailOtp.length !== 6} onClick={() => otpVerify.mutate()}><CheckCircle2 size={14} />校验 OTP</Button>
            </div>
          </div>
        )}
      </div>
    </section>
  );
}

function submit(event: FormEvent<HTMLFormElement>, run: () => void) { event.preventDefault(); run(); }

function shouldShowEmailOtp(status?: AccountSettingsOperationStatus) {
  return status === AccountSettingsOperationStatus.ACCOUNT_SETTINGS_OPERATION_STATUS_NEEDS_VERIFICATION
    || status === AccountSettingsOperationStatus.ACCOUNT_SETTINGS_OPERATION_STATUS_WAITING
    || status === AccountSettingsOperationStatus.ACCOUNT_SETTINGS_OPERATION_STATUS_CODE_MISMATCH;
}

function twoFactorStatusLabel(query: { isPending: boolean; isError: boolean; data?: { status?: { configured?: boolean } } }) {
  if (query.isPending) return '读取中';
  if (query.isError) return '读取失败';
  return query.data?.status?.configured ? '已配置' : '未配置';
}

function twoFactorBadgeVariant(query: { isPending: boolean; isError: boolean; data?: { status?: { configured?: boolean } } }): BadgeVariant {
  if (query.isError) return 'destructive';
  return query.data?.status?.configured ? 'default' : 'outline';
}

function statusLabel(status?: AccountSettingsOperationStatus) {
  switch (status) {
    case AccountSettingsOperationStatus.ACCOUNT_SETTINGS_OPERATION_STATUS_NEEDS_VERIFICATION: return '待邮箱验证';
    case AccountSettingsOperationStatus.ACCOUNT_SETTINGS_OPERATION_STATUS_WAITING: return '等待 OTP';
    case AccountSettingsOperationStatus.ACCOUNT_SETTINGS_OPERATION_STATUS_VERIFIED: return '已验证';
    case AccountSettingsOperationStatus.ACCOUNT_SETTINGS_OPERATION_STATUS_CODE_MISMATCH: return '验证码不匹配';
    case AccountSettingsOperationStatus.ACCOUNT_SETTINGS_OPERATION_STATUS_REJECTED: return '已拒绝';
    case AccountSettingsOperationStatus.ACCOUNT_SETTINGS_OPERATION_STATUS_ACCEPTED: return '已受理';
    default: return '未执行';
  }
}
