/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { useState } from 'react'
import { Download, ExternalLink, Film } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Dialog } from '@/components/dialog'

interface VideoPreviewCellProps {
  taskId: string
  isAdmin: boolean
}

function VideoPlayer(props: { src: string }) {
  const { t } = useTranslation()
  const [loading, setLoading] = useState(true)
  const [failed, setFailed] = useState(false)

  return (
    <div className='space-y-2'>
      <video
        src={props.src}
        controls
        playsInline
        preload='metadata'
        aria-label={t('Video Preview')}
        className='max-h-[55vh] w-full rounded-md bg-black object-contain'
        onLoadedData={() => setLoading(false)}
        onError={() => {
          setLoading(false)
          setFailed(true)
        }}
      />
      {loading && (
        <p role='status' className='text-muted-foreground text-sm'>
          {t('Loading...')}
        </p>
      )}
      {failed && (
        <p role='alert' className='text-destructive text-sm'>
          {t('Video playback failed')}
        </p>
      )}
    </div>
  )
}

export function VideoPreviewCell(props: VideoPreviewCellProps) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  const taskId = encodeURIComponent(props.taskId)
  const src = props.isAdmin
    ? '/api/task/' + taskId + '/content'
    : '/v1/videos/' + taskId + '/content'

  return (
    <>
      <button
        type='button'
        className='group flex items-center gap-1 text-left text-xs'
        onClick={() => setOpen(true)}
      >
        <Film className='text-muted-foreground size-3' aria-hidden='true' />
        <span className='text-foreground leading-snug group-hover:underline'>
          {t('Click to preview video')}
        </span>
      </button>
      <Dialog
        open={open}
        onOpenChange={setOpen}
        title={t('Video Preview')}
        description={props.taskId}
        descriptionClassName='break-all font-mono text-xs'
        contentClassName='sm:max-w-3xl'
        footer={
          <div className='flex flex-wrap gap-4 text-sm'>
            <a
              href={src}
              target='_blank'
              rel='noopener noreferrer'
              className='text-primary inline-flex items-center gap-1 hover:underline'
            >
              <ExternalLink className='size-4' aria-hidden='true' />
              {t('Open in new tab')}
            </a>
            <a
              href={src}
              download={props.taskId + '.mp4'}
              className='text-primary inline-flex items-center gap-1 hover:underline'
            >
              <Download className='size-4' aria-hidden='true' />
              {t('Download')}
            </a>
          </div>
        }
      >
        {open && <VideoPlayer key={src} src={src} />}
      </Dialog>
    </>
  )
}
