import { Button } from './button'
import { Icon } from './icon'
import { Modal } from './modal'

type ConfirmDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  title: string
  description: string
  confirmLabel: string
  cancelLabel: string
  closeLabel: string
  onConfirm: () => void
  pending?: boolean
  destructive?: boolean
}

export function ConfirmDialog({
  open,
  onOpenChange,
  title,
  description,
  confirmLabel,
  cancelLabel,
  closeLabel,
  onConfirm,
  pending = false,
  destructive = false,
}: ConfirmDialogProps) {
  return (
    <Modal
      open={open}
      onClose={() => !pending && onOpenChange(false)}
      title={title}
      description={description}
      size="sm"
      closeLabel={closeLabel}
      footer={(
        <>
          <Button size="sm" variant="outline" type="button" disabled={pending} onClick={() => onOpenChange(false)}>{cancelLabel}</Button>
          <Button size="sm" variant={destructive ? 'destructive' : 'default'} type="button" disabled={pending} onClick={onConfirm}>
            {pending ? <Icon name="loader" className="h-4 w-4 animate-spin" /> : destructive ? <Icon name="trash" /> : <Icon name="check" />}
            {confirmLabel}
          </Button>
        </>
      )}
    />
  )
}
