import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import type { Image } from '../../lib/api-types'
import { readLocalStorage, writeLocalStorage } from '../../lib/browser-storage'
import { copyText, imageLinks, type LinkFormat } from '../../lib/image-links'
import { Button } from './button'
import { DropdownItem, DropdownMenu } from './dropdown-menu'
import { Icon, type IconName } from './icon'

const formats: LinkFormat[] = ['direct', 'markdown', 'bbcode', 'html']
const formatIcons = {
  direct: 'link',
  markdown: 'fileText',
  bbcode: 'brackets',
  html: 'code',
} satisfies Record<LinkFormat, IconName>
const storageKey = 'imagesilo_link_format'

function initialFormat(): LinkFormat {
  const value = readLocalStorage(storageKey)
  return formats.includes(value as LinkFormat) ? (value as LinkFormat) : 'direct'
}

export function CopyLinkControl({ image, compact = false, onCopied }: { image: Image; compact?: boolean; onCopied?: (format: LinkFormat) => void }) {
  const { t } = useTranslation()
  const [format, setFormat] = useState<LinkFormat>(initialFormat)
  const [open, setOpen] = useState(false)
  const [copying, setCopying] = useState(false)
  const links = imageLinks(image)

  useEffect(() => {
    writeLocalStorage(storageKey, format)
  }, [format])

  async function copy() {
    setCopying(true)
    try {
      await copyText(links[format])
      toast.success(t('toast.linkCopied', { format: t(`images.linkFormatShort.${format}`) }))
      onCopied?.(format)
    } catch {
      toast.error(t('toast.copyFailed'))
    } finally {
      setCopying(false)
    }
  }

  return (
    <div className="copy-link-control" data-compact={compact || undefined} onClick={(event) => event.stopPropagation()}>
      <Button
        className="copy-link-main"
        size={compact ? 'xs' : 'sm'}
        variant="outline"
        type="button"
        disabled={copying}
        aria-label={t('images.copySelectedFormat', { format: t(`images.linkFormatShort.${format}`) })}
        onClick={() => void copy()}
      >
        <Icon name={copying ? 'loader' : 'copy'} className={copying ? 'h-4 w-4 animate-spin' : 'h-4 w-4'} />
        <span className="image-action-label">{t('common.copy')}</span>
      </Button>
      <DropdownMenu
        open={open}
        onOpenChange={setOpen}
        align="right"
        className="copy-format-menu"
        trigger={
          <button
            className="copy-link-caret"
            type="button"
            title={t(`images.linkFormat.${format}`)}
            aria-label={`${t('images.chooseLinkFormat')}: ${t(`images.linkFormat.${format}`)}`}
            aria-expanded={open}
            onClick={() => setOpen((value) => !value)}
          >
            <Icon name={formatIcons[format]} className="copy-format-icon" />
            <Icon name="chevronDown" className="copy-format-chevron" />
          </button>
        }
      >
        {formats.map((value) => (
          <DropdownItem
            key={value}
            active={value === format}
            onClick={() => {
              setFormat(value)
              setOpen(false)
            }}
          >
            <Icon name={formatIcons[value]} />
            <span>{t(`images.linkFormat.${value}`)}</span>
            {value === format ? <Icon name="check" className="ml-auto h-4 w-4 text-brand-500" /> : null}
          </DropdownItem>
        ))}
      </DropdownMenu>
    </div>
  )
}
