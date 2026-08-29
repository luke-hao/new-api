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
import { type TFunction } from 'i18next'
import {
  Box,
  CreditCard,
  Layout,
  Settings,
  Shield,
  ShieldAlert,
  Wrench,
} from 'lucide-react'
import { SYSTEM_SETTINGS_SECTIONS } from '@/features/system-settings/section-metadata'
import type { NavGroup, SidebarView } from '../types'

type SettingsArea = keyof typeof SYSTEM_SETTINGS_SECTIONS

function getSectionNavItems(t: TFunction, area: SettingsArea) {
  return SYSTEM_SETTINGS_SECTIONS[area].map(([id, titleKey]) => ({
    title: t(titleKey),
    url: `/system-settings/${area}/${id}`,
  }))
}

/**
 * Sidebar nav groups for the System Settings nested view.
 *
 * Kept as a single group because the workspace title in the sidebar
 * header already provides top-level context — the inner group label
 * scopes the items as "administration" actions.
 */
function getSystemSettingsNavGroups(t: TFunction): NavGroup[] {
  return [
    {
      id: 'system-administration',
      title: t('System Administration'),
      items: [
        {
          title: t('Site & Branding'),
          icon: Settings,
          items: getSectionNavItems(t, 'site'),
        },
        {
          title: t('Authentication'),
          icon: Shield,
          items: getSectionNavItems(t, 'auth'),
        },
        {
          title: t('Billing & Payment'),
          icon: CreditCard,
          items: getSectionNavItems(t, 'billing'),
        },
        {
          title: t('Models & Routing'),
          icon: Box,
          items: getSectionNavItems(t, 'models'),
        },
        {
          title: t('Security & Limits'),
          icon: ShieldAlert,
          items: getSectionNavItems(t, 'security'),
        },
        {
          title: t('Console Content'),
          icon: Layout,
          items: getSectionNavItems(t, 'content'),
        },
        {
          title: t('Operations'),
          icon: Wrench,
          items: getSectionNavItems(t, 'operations'),
        },
      ],
    },
  ]
}

/**
 * Nested sidebar view for `/system-settings/*`.
 *
 * Activates the Vercel / Cloudflare-style drill-in sidebar:
 * the root navigation is replaced by the system administration
 * groups, with a "Back to Dashboard" affordance in the header.
 */
export const SYSTEM_SETTINGS_VIEW: SidebarView = {
  id: 'system-settings',
  pathPattern: /^\/system-settings(\/|$)/,
  parent: {
    to: '/dashboard/overview',
    label: 'Back to Dashboard',
  },
  getNavGroups: getSystemSettingsNavGroups,
}
