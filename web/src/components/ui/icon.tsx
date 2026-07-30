import {
  Activity,
  ArrowRight,
  Brackets,
  CalendarDays,
  Check,
  ChevronDown,
  ChevronLeft,
  ChevronRight,
  CloudUpload,
  CodeXml,
  Command,
  Copy,
  ExternalLink,
  Eye,
  EyeOff,
  FileText,
  Filter,
  Globe2,
  Grid2X2,
  History,
  Image,
  Images,
  KeyRound,
  Languages,
  Link,
  List,
  LoaderCircle,
  LockKeyhole,
  LogOut,
  Mail,
  Menu,
  Moon,
  Plus,
  RefreshCw,
  Search,
  Server,
  Settings,
  ShieldCheck,
  Sparkles,
  Sun,
  Trash2,
  Upload,
  UserRound,
  WandSparkles,
  X,
  Zap,
  type LucideIcon,
} from 'lucide-react'

export type IconName =
  | 'activity'
  | 'arrowRight'
  | 'brackets'
  | 'calendar'
  | 'check'
  | 'chevronDown'
  | 'chevronLeft'
  | 'chevronRight'
  | 'cloudUpload'
  | 'code'
  | 'command'
  | 'copy'
  | 'external'
  | 'fileText'
  | 'filter'
  | 'globe'
  | 'grid'
  | 'history'
  | 'image'
  | 'images'
  | 'key'
  | 'languages'
  | 'link'
  | 'list'
  | 'loader'
  | 'lock'
  | 'logOut'
  | 'mail'
  | 'menu'
  | 'moon'
  | 'plus'
  | 'refresh'
  | 'search'
  | 'server'
  | 'settings'
  | 'shield'
  | 'sparkles'
  | 'sun'
  | 'trash'
  | 'upload'
  | 'user'
  | 'visibility'
  | 'visibilityOff'
  | 'wand'
  | 'x'
  | 'zap'

const icons: Record<IconName, LucideIcon> = {
  activity: Activity,
  arrowRight: ArrowRight,
  brackets: Brackets,
  calendar: CalendarDays,
  check: Check,
  chevronDown: ChevronDown,
  chevronLeft: ChevronLeft,
  chevronRight: ChevronRight,
  cloudUpload: CloudUpload,
  code: CodeXml,
  command: Command,
  copy: Copy,
  external: ExternalLink,
  fileText: FileText,
  filter: Filter,
  globe: Globe2,
  grid: Grid2X2,
  history: History,
  image: Image,
  images: Images,
  key: KeyRound,
  languages: Languages,
  link: Link,
  list: List,
  loader: LoaderCircle,
  lock: LockKeyhole,
  logOut: LogOut,
  mail: Mail,
  menu: Menu,
  moon: Moon,
  plus: Plus,
  refresh: RefreshCw,
  search: Search,
  server: Server,
  settings: Settings,
  shield: ShieldCheck,
  sparkles: Sparkles,
  sun: Sun,
  trash: Trash2,
  upload: Upload,
  user: UserRound,
  visibility: Eye,
  visibilityOff: EyeOff,
  wand: WandSparkles,
  x: X,
  zap: Zap,
}

export function Icon({ name, className = 'h-4 w-4' }: { name: IconName; className?: string }) {
  const Component = icons[name]
  return <Component aria-hidden="true" className={className} strokeWidth={1.8} />
}
