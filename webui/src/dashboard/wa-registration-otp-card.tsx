import { KeyRound } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardTitle } from '@/components/ui/card';
import { FieldDescription } from '@/components/ui/field';
import { Input } from '@/components/ui/input';

type Props = {
  phone: string;
  verificationRequestID: string;
  value: string;
  busy?: boolean;
  onChange: (value: string) => void;
  onSubmit: () => void;
};

export function WaRegistrationOtpCard({ phone, verificationRequestID, value, busy, onChange, onSubmit }: Props) {
  return (
    <Card className="border-dashed">
      <CardContent className="grid gap-2 p-3">
        <CardTitle className="inline-flex items-center gap-2 text-sm"><KeyRound size={15} />输入注册 OTP</CardTitle>
        <div className="flex gap-2">
          <Input value={value} onChange={(event) => onChange(event.target.value)} inputMode="numeric" autoComplete="one-time-code" type="password" placeholder="验证码" disabled={busy} />
          <Button type="button" disabled={busy || !value.trim()} onClick={onSubmit}>提交</Button>
        </div>
        <FieldDescription>{phone}{verificationRequestID ? ` · ${verificationRequestID}` : ''}</FieldDescription>
      </CardContent>
    </Card>
  );
}
