import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { zodResolver } from '@hookform/resolvers/zod'
import { useForm } from 'react-hook-form'
import { z } from 'zod'

import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Textarea } from '@/components/ui/textarea'
import { CHANNEL_TYPE_OPTIONS } from '@/features/channels/constants'
import { handleServerError } from '@/lib/handle-server-error'

import { submitChannel } from '../api'

// Channel types closed to contributor onboarding (mirrors the backend
// constant.IsContributorAllowedType blocklist): Midjourney variants, Vertex
// AI (service-account JSON), Codex (OAuth), task plugins (internal).
const BLOCKED_TYPES = new Set([2, 5, 41, 57, 61])

const submitSchema = z.object({
  type: z.number().min(1, 'Channel type is required'),
  key: z.string().min(1, 'API key is required'),
  name: z.string().min(1, 'Name is required').max(100),
  base_url: z.string().optional(),
  models: z.string().min(1, 'At least one model is required'),
})

type SubmitForm = z.infer<typeof submitSchema>

interface SubmitChannelDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  onSubmitted: () => void
}

export function SubmitChannelDialog(props: SubmitChannelDialogProps) {
  const { t } = useTranslation()
  const [submitting, setSubmitting] = useState(false)
  const allowedTypes = CHANNEL_TYPE_OPTIONS.filter((o) => !BLOCKED_TYPES.has(o.value))

  const form = useForm<SubmitForm>({
    resolver: zodResolver(submitSchema),
    defaultValues: { type: 1, key: '', name: '', base_url: '', models: '' },
  })

  const onSubmit = async (values: SubmitForm) => {
    setSubmitting(true)
    try {
      await submitChannel({
        type: values.type,
        key: values.key.trim(),
        name: values.name.trim(),
        base_url: values.base_url?.trim() || null,
        models: values.models.trim(),
      })
      form.reset()
      props.onOpenChange(false)
      props.onSubmitted()
    } catch (error) {
      handleServerError(error, t('Failed to submit channel'))
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <Dialog
      open={props.open}
      onOpenChange={props.onOpenChange}
      title={t('Submit a channel')}
      description={t(
        'Share an API key channel. It goes live after review. Your key is encrypted and never shown back to you.',
      )}
    >
      <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-4">
        <div className="space-y-2">
          <Label htmlFor="cc-type">{t('Channel type')}</Label>
          <Select
            value={String(form.watch('type'))}
            onValueChange={(v) => form.setValue('type', Number(v))}
          >
            <SelectTrigger id="cc-type">
              <SelectValue placeholder={t('Select a channel type')} />
            </SelectTrigger>
            <SelectContent>
              {allowedTypes.map((o) => (
                <SelectItem key={o.value} value={String(o.value)}>
                  {o.label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>

        <div className="space-y-2">
          <Label htmlFor="cc-key">{t('API key')}</Label>
          <Textarea
            id="cc-key"
            placeholder="sk-..."
            rows={3}
            {...form.register('key')}
          />
          {form.formState.errors.key && (
            <p className="text-sm text-destructive">{form.formState.errors.key.message}</p>
          )}
        </div>

        <div className="space-y-2">
          <Label htmlFor="cc-name">{t('Name')}</Label>
          <Input
            id="cc-name"
            placeholder={t('A name to identify your channel')}
            {...form.register('name')}
          />
          {form.formState.errors.name && (
            <p className="text-sm text-destructive">{form.formState.errors.name.message}</p>
          )}
        </div>

        <div className="space-y-2">
          <Label htmlFor="cc-base-url">{t('Base URL (optional)')}</Label>
          <Input
            id="cc-base-url"
            placeholder="https://api.openai.com/v1"
            {...form.register('base_url')}
          />
        </div>

        <div className="space-y-2">
          <Label htmlFor="cc-models">{t('Models (comma separated)')}</Label>
          <Input
            id="cc-models"
            placeholder="gpt-4o-mini, gpt-4o"
            {...form.register('models')}
          />
          {form.formState.errors.models && (
            <p className="text-sm text-destructive">{form.formState.errors.models.message}</p>
          )}
        </div>

        <div className="flex justify-end gap-2 pt-2">
          <Button
            type="button"
            variant="outline"
            onClick={() => props.onOpenChange(false)}
            disabled={submitting}
          >
            {t('Cancel')}
          </Button>
          <Button type="submit" disabled={submitting}>
            {submitting ? t('Submitting...') : t('Submit for review')}
          </Button>
        </div>
      </form>
    </Dialog>
  )
}
