import { createFileRoute, redirect } from '@tanstack/react-router'
import { isSidebarModuleEnabled } from '@/lib/nav-modules'
import { Main } from '@/components/layout'
import { MediaStudio } from '@/features/media-studio'

export const Route = createFileRoute('/_authenticated/studio/')({
  beforeLoad: () => {
    if (!isSidebarModuleEnabled('chat', 'mediaStudio')) {
      throw redirect({ to: '/dashboard' })
    }
  },
  component: StudioPage,
})

function StudioPage() {
  return (
    <Main className='p-0'>
      <MediaStudio />
    </Main>
  )
}
