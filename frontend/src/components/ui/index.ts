/**
 * UI Component Library
 *
 * Reusable components with built-in dark mode support using CSS variables.
 * All components use semantic color classes (surface-base, text-content-primary, etc.)
 * instead of manual dark: classes.
 *
 * @example
 * ```tsx
 * import { Button, Input, Card } from '@/components/ui';
 *
 * function MyComponent() {
 *   return (
 *     <Card variant="elevated" padding="md">
 *       <Input label="Email" type="email" />
 *       <Button variant="primary">Submit</Button>
 *     </Card>
 *   );
 * }
 * ```
 */

// Button
export { Button } from './Button';
export type { ButtonProps, ButtonVariant, ButtonSize } from './Button';

// Input
export { Input } from './Input';
export type { InputProps, InputSize, InputVariant } from './Input';

// Card
export { Card, CardHeader, CardBody, CardFooter } from './Card';
export type {
  CardProps,
  CardVariant,
  CardPadding,
  CardHeaderProps,
  CardBodyProps,
  CardFooterProps,
} from './Card';

// Badge
export { Badge } from './Badge';
export type { BadgeProps, BadgeVariant, BadgeSize } from './Badge';

// Alert
export { Alert } from './Alert';
export type { AlertProps, AlertVariant } from './Alert';

// Spinner
export { Spinner } from './Spinner';
export type { SpinnerProps, SpinnerSize, SpinnerVariant } from './Spinner';

// Label
export { Label } from './Label';
export type { LabelProps } from './Label';

// Checkbox
export { Checkbox } from './Checkbox';
export type { CheckboxProps } from './Checkbox';

// Radio
export { Radio } from './Radio';
export type { RadioProps } from './Radio';

// Toggle
export { Toggle } from './Toggle';
export type { ToggleProps, ToggleSize } from './Toggle';

// Textarea
export { Textarea } from './Textarea';
export type { TextareaProps, TextareaSize, TextareaVariant } from './Textarea';

// Select
export { Select } from './Select';
export type { SelectProps, SelectSize, SelectVariant } from './Select';

// DateTimeInput
export { DateTimeInput } from './DateTimeInput';
export type { DateTimeInputProps, DateTimeInputSize, DateTimeInputVariant } from './DateTimeInput';

// Modal
export { Modal } from './Modal';
export type { ModalProps } from './Modal';

// MetadataItem
export { MetadataItem, MetadataGroup, MetadataSeparator } from './MetadataItem';
export type { MetadataItemProps } from './MetadataItem';

// HelpTooltip
export { HelpTooltip } from './HelpTooltip';

// Drawer
export { Drawer } from './Drawer';
export type { DrawerProps } from './Drawer';
