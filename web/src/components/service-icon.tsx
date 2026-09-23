import type { ReactNode } from 'react'

export type ServiceIconName = 'building' | 'user' | 'shield' | 'book' | 'code' | 'trophy' | 'list' | 'arrow' | 'notice'

export function ServiceIcon({ name, className = '' }: { name: ServiceIconName; className?: string }) {
  const paths: Record<ServiceIconName, ReactNode> = {
    building: <><path d="m3 9 9-6 9 6H3Zm2 0v11m14-11v11M9 12v8m6-8v8M3 21h18" /><path d="M11 6h2" /></>,
    user: <><circle cx="12" cy="8" r="4" /><path d="M4 21v-2a8 8 0 0 1 16 0v2M9 17l3 3 3-3" /></>,
    shield: <><path d="m12 3 8 3v6c0 5-8 9-8 9s-8-4-8-9V6l8-3Z" /><path d="m8 12 3 3 5-6" /></>,
    book: <><path d="M12 5v16M3 4h5a4 4 0 0 1 4 4 4 4 0 0 1 4-4h5v15h-5a4 4 0 0 0-4 2 4 4 0 0 0-4-2H3V4Z" /></>,
    code: <><rect x="3" y="4" width="18" height="16" rx="1" /><path d="m9 9-3 3 3 3m6-6 3 3-3 3m-2-7-2 10" /></>,
    trophy: <><path d="M8 3h8v7a4 4 0 0 1-8 0V3Zm8 2h4v3a4 4 0 0 1-4 4M8 5H4v3a4 4 0 0 0 4 4m4 2v5m-5 2h10m-8-2h6" /></>,
    list: <><rect x="5" y="3" width="14" height="18" rx="1" /><path d="M9 8h6m-6 4h6m-6 4h4" /></>,
    arrow: <path d="M4 12h16m-6-6 6 6-6 6" />,
    notice: <><path d="m4 10 12-5v14L4 14v-4Zm3 5 2 6h3l-2-5m10-7 2-1m-2 8 2 1" /></>,
  }
  return <svg className={`service-icon ${className}`} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">{paths[name]}</svg>
}

export function ServiceHallIllustration() {
  return <svg className="hall-illustration" viewBox="0 0 600 310" fill="none" aria-hidden="true">
    <circle cx="352" cy="147" r="125" fill="#f2e7d8" />
    <g stroke="#c9b99f" strokeWidth="1.3">
      <path d="M12 275h573M29 284h520M58 257V126h60v131m-52-112h44m-44 18h44m-44 18h44m-44 18h44m-44 18h44m-44 18h44M480 261V109h52v152m-42-138h32m-32 18h32m-32 18h32m-32 18h32m-32 18h32m-32 18h32m-32 18h32" opacity=".5" />
      <path d="M138 249V127l162-76 162 76v122" fill="#fbf6ec" />
      <path d="m120 132 180-87 180 87H120Z" fill="#ede0ca" />
      <path d="m146 122 154-64 154 64H146ZM148 144h304v13H148z" />
      <path d="M154 157h27v84h-27zm55 0h27v84h-27zm55 0h27v84h-27zm55 0h27v84h-27zm55 0h27v84h-27zm55 0h17v84h-17z" fill="#f0e3cc" />
      <path d="M164 163v72m55-72v72m55-72v72m55-72v72m55-72v72m55-72v72" />
      <path d="M144 241h312v10H144zm-12 10h336v10H132zm-12 10h360v10H120z" fill="#f3e8d7" />
      <circle cx="300" cy="99" r="13" /><path d="M294 99h12m-6-6v12" />
      <path d="M88 271v-30m-14 3c-14-25 12-46 16-46 4 0 25 25 14 45-5 8-23 9-30 1Zm429 27v-30m-14 3c-14-25 12-46 16-46 4 0 25 25 14 45-5 8-23 9-30 1Z" fill="#e9e4d4" />
    </g>
    <path d="M300 45V19m0 0c16-7 24 8 39 1v20c-15 7-23-8-39-1" stroke="#ab383b" fill="#b84143" strokeWidth="1.5" />
  </svg>
}
