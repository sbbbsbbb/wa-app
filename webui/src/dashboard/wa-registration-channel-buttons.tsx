import { CheckCircle2, Clock3, CircleDashed, XCircle } from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { countdownLabel } from './wa-result-labels';
import {
  registrationMethodAvailable,
  registrationMethodCooldownSeconds,
  registrationMethodStatus,
  selectableRegistrationMethods,
  type SelectableRegistrationMethodOption,
} from './wa-registration-methods';
import type { WaProbeStatus } from './wa-result-model';

type Props = {
  status: WaProbeStatus | null;
  elapsedSeconds: number;
  disabled?: boolean;
  onStart: (method: SelectableRegistrationMethodOption) => void;
};

export function WaRegistrationChannelButtons({ status, elapsedSeconds, disabled, onStart }: Props) {
  const methods = status ? selectableRegistrationMethods.filter((method) => registrationMethodStatus(status, method.value)) : selectableRegistrationMethods;
  return (
    <div className="grid gap-2 sm:grid-cols-2">
      {methods.map((method) => {
        const state = channelState(method, status, elapsedSeconds);
        return (
          <Button
            key={method.value}
            type="button"
            variant={state.ready ? 'default' : state.cooldown > 0 ? 'secondary' : 'outline'}
            className="h-auto min-h-14 justify-between gap-3 px-3 py-2 text-left"
            disabled={disabled || !state.ready}
            aria-label={`${method.label} ${state.label}`}
            title={state.title || method.description}
            onClick={() => onStart(method)}
          >
            <span className="grid min-w-0 gap-0.5">
              <span className="truncate text-sm font-semibold">{method.label}</span>
              <span className="truncate text-[11px] font-normal opacity-80">{method.description}</span>
            </span>
            <Badge variant={state.badge} className="shrink-0">
              <state.Icon />
              {state.label}
            </Badge>
          </Button>
        );
      })}
    </div>
  );
}

function channelState(method: SelectableRegistrationMethodOption, status: WaProbeStatus | null, elapsedSeconds: number) {
  if (!status) return { ready: false, cooldown: 0, label: '先检测', badge: 'outline' as const, Icon: CircleDashed, title: '先检测号码，再选择可用通道' };
  const cooldown = registrationMethodCooldownSeconds(status, method.value, elapsedSeconds);
  if (cooldown > 0) {
    return { ready: false, cooldown, label: countdownLabel(cooldown), badge: 'secondary' as const, Icon: Clock3, title: `冷却中，剩余 ${countdownLabel(cooldown)}` };
  }
  if (registrationMethodAvailable(status, method.value, elapsedSeconds)) {
    return { ready: true, cooldown: 0, label: '可用', badge: 'default' as const, Icon: CheckCircle2, title: method.description };
  }
  return { ready: false, cooldown: 0, label: '不可用', badge: 'outline' as const, Icon: XCircle, title: `${method.label} 当前不可用` };
}
