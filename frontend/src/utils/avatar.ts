export function getInitials(name: string): string {
  return name
    .split(' ')
    .map(n => n.charAt(0))
    .join('')
    .toUpperCase()
    .slice(0, 2)
}

export function getAvatarColor(name: string): string {
  // Tinted warm neutrals: identity carries the initials, not competing hues.
  // The single accent stays the only voice in the system.
  const colors = [
    'bg-amber-950/10 text-amber-900 dark:bg-amber-300/10 dark:text-amber-100',
    'bg-orange-950/10 text-orange-900 dark:bg-orange-300/10 dark:text-orange-100',
    'bg-rose-950/10 text-rose-900 dark:bg-rose-300/10 dark:text-rose-100',
    'bg-stone-200/70 text-stone-700 dark:bg-stone-800 dark:text-stone-200',
    'bg-neutral-200/70 text-neutral-700 dark:bg-neutral-800 dark:text-neutral-200',
  ]
  let hash = 0
  for (let i = 0; i < name.length; i++) {
    hash = name.charCodeAt(i) + ((hash << 5) - hash)
  }
  return colors[Math.abs(hash) % colors.length]
}
