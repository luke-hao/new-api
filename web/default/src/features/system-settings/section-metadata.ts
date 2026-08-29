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
export const SYSTEM_SETTINGS_SECTIONS = {
  site: [
    ['system-info', 'System Information'],
    ['notice', 'System Notice'],
    ['header-navigation', 'Header navigation'],
    ['sidebar-modules', 'Sidebar modules'],
  ],
  auth: [
    ['basic-auth', 'Basic Authentication'],
    ['oauth', 'OAuth Integrations'],
    ['passkey', 'Passkey Authentication'],
    ['bot-protection', 'Bot Protection'],
    ['custom-oauth', 'Custom OAuth'],
  ],
  billing: [
    ['quota', 'Quota Settings'],
    ['currency', 'Currency & Display'],
    ['model-pricing', 'Model Pricing'],
    ['group-pricing', 'Group Pricing'],
    ['payment', 'Payment Gateway'],
    ['checkin', 'Check-in Rewards'],
  ],
  models: [
    ['global', 'Global Model Configuration'],
    ['gemini', 'Gemini'],
    ['claude', 'Claude'],
    ['grok', 'Grok'],
    ['channel-affinity', 'Channel Affinity'],
    ['model-deployment', 'Model Deployment'],
  ],
  security: [
    ['rate-limit', 'Rate Limiting'],
    ['sensitive-words', 'Sensitive Words'],
    ['ssrf', 'SSRF Protection'],
  ],
  content: [
    ['dashboard', 'Data Dashboard'],
    ['announcements', 'Announcements'],
    ['api-info', 'API Addresses'],
    ['faq', 'FAQ'],
    ['uptime-kuma', 'Uptime Kuma'],
    ['chat', 'Chat Presets'],
    ['drawing', 'Drawing'],
  ],
  operations: [
    ['behavior', 'System Behavior'],
    ['monitoring', 'Monitoring & Alerts'],
    ['email', 'SMTP Email'],
    ['worker', 'Worker Proxy'],
    ['logs', 'Log Maintenance'],
    ['performance', 'Performance'],
    ['update-checker', 'System maintenance'],
  ],
} as const

export const SITE_SECTION_IDS = SYSTEM_SETTINGS_SECTIONS.site.map(([id]) => id)
export const SITE_DEFAULT_SECTION = SITE_SECTION_IDS[0]
export const AUTH_SECTION_IDS = SYSTEM_SETTINGS_SECTIONS.auth.map(([id]) => id)
export const AUTH_DEFAULT_SECTION = AUTH_SECTION_IDS[0]
export const BILLING_SECTION_IDS = SYSTEM_SETTINGS_SECTIONS.billing.map(
  ([id]) => id
)
export const BILLING_DEFAULT_SECTION = BILLING_SECTION_IDS[0]
export const MODELS_SECTION_IDS = SYSTEM_SETTINGS_SECTIONS.models.map(
  ([id]) => id
)
export const MODELS_DEFAULT_SECTION = MODELS_SECTION_IDS[0]
export const SECURITY_SECTION_IDS = SYSTEM_SETTINGS_SECTIONS.security.map(
  ([id]) => id
)
export const SECURITY_DEFAULT_SECTION = SECURITY_SECTION_IDS[0]
export const CONTENT_SECTION_IDS = SYSTEM_SETTINGS_SECTIONS.content.map(
  ([id]) => id
)
export const CONTENT_DEFAULT_SECTION = CONTENT_SECTION_IDS[0]
export const OPERATIONS_SECTION_IDS = SYSTEM_SETTINGS_SECTIONS.operations.map(
  ([id]) => id
)
export const OPERATIONS_DEFAULT_SECTION = OPERATIONS_SECTION_IDS[0]
