import { Badge } from '@/components/ui/badge';
import { apkSupportedLoginRegistrationMethods } from './wa-registration-methods';

export function WaRegistrationMethodList() {
  return (
    <div className="flex flex-wrap gap-1.5">
      {apkSupportedLoginRegistrationMethods.map((method) => (
        <Badge key={method.value} variant="outline" title={method.description}>{method.label}</Badge>
      ))}
    </div>
  );
}
