import React from 'react'

type H = React.ComponentType<React.SVGProps<SVGSVGElement>>

function h(Icon: H) {
  return React.forwardRef<SVGSVGElement, any>(({ size, fontSize, className, ...props }, ref) => {
    const numericFontSize = typeof fontSize === 'number' ? fontSize : undefined;
    const s = size || numericFontSize || 24
    return <Icon ref={ref} width={s} height={s} className={className} {...props} />
  })
}

import {
  ArrowDownIcon, ArrowDownRightIcon, ArrowRightIcon, ArrowUpIcon, ArrowUpRightIcon,
  ArrowPathIcon, ArrowPathRoundedSquareIcon, ArrowTopRightOnSquareIcon,
  ArrowDownTrayIcon,
  Bars3Icon, BellAlertIcon, BoltIcon,
  CheckBadgeIcon, CheckCircleIcon, ChevronDownIcon, ChevronUpIcon,
  ClockIcon, CloudIcon, CodeBracketIcon, ComputerDesktopIcon,
  CogIcon, CpuChipIcon,
  DocumentCheckIcon, DocumentTextIcon,
  ExclamationCircleIcon,
  FolderOpenIcon, FunnelIcon,
  GlobeAltIcon,
  HeartIcon,
  InformationCircleIcon,
  LanguageIcon, LifebuoyIcon, LinkIcon as HeroLinkIcon, LockClosedIcon,
  MagnifyingGlassIcon, MapIcon, MegaphoneIcon, MinusIcon, MoonIcon,
  PauseIcon, PencilIcon, PlayIcon, PlusIcon, PlusCircleIcon, PowerIcon,
  RectangleGroupIcon,
  ServerIcon, ShareIcon, ShieldCheckIcon, ShieldExclamationIcon,
  SignalIcon, SparklesIcon, Squares2X2Icon, StopIcon, SunIcon,
  TrashIcon,
  UserGroupIcon,
  WifiIcon,
  XCircleIcon, XMarkIcon,
} from '@heroicons/react/24/outline'

import {
  CheckBadgeIcon as CheckSquareSolid,
} from '@heroicons/react/24/solid'

export const Activity = h(RectangleGroupIcon)
export const Add = h(PlusIcon)
export const AddCircle = h(PlusCircleIcon)
export const AlertCircle = h(ExclamationCircleIcon)
export const Anchor = h(LifebuoyIcon)
export const Antenna = h(SignalIcon)
export const ArrowDown = h(ArrowDownIcon)
export const ArrowDownRight = h(ArrowDownRightIcon)
export const ArrowDownward = h(ArrowDownIcon)
export const ArrowForward = h(ArrowRightIcon)
export const ArrowRight = h(ArrowRightIcon)
export const ArrowUp = h(ArrowUpIcon)
export const ArrowUpRight = h(ArrowUpRightIcon)
export const AutoAwesome = h(SparklesIcon)
export const BellRing = h(BellAlertIcon)
export const Bolt = h(BoltIcon)
export const Campaign = h(MegaphoneIcon)
export const Cancel = h(XCircleIcon)
export const CheckCircle = h(CheckCircleIcon)
export const CheckCircle2 = h(CheckCircleIcon)
export const CheckSquare = h(CheckSquareSolid)
export const ChevronDown = h(ChevronDownIcon)
export const ChevronsUp = h(ChevronUpIcon)
export const ChevronUp = h(ChevronUpIcon)
export const Cloud = h(CloudIcon)
export const CloudLightning = h(BoltIcon)
export const Code = h(CodeBracketIcon)
export const Code2 = h(CodeBracketIcon)
export const Cpu = h(CpuChipIcon)
export const Delete = h(TrashIcon)
export const Description = h(DocumentTextIcon)
export const Download = h(ArrowDownTrayIcon)
export const Edit = h(PencilIcon)
export const Edit3 = h(PencilIcon)
export const Error = h(ExclamationCircleIcon)
export const ExternalLink = h(ArrowTopRightOnSquareIcon)
export const Favorite = h(HeartIcon)
export const FileText = h(DocumentTextIcon)
export const Filter = h(FunnelIcon)
export const FolderOpen = h(FolderOpenIcon)
export const GitBranch = h(ArrowPathRoundedSquareIcon)
export const Globe = h(GlobeAltIcon)
export const Heart = h(HeartIcon)
export const History = h(ClockIcon)
export const Info = h(InformationCircleIcon)
export const Language = h(LanguageIcon)
export const Languages = h(LanguageIcon)
export const LayoutDashboard = h(RectangleGroupIcon)
export const Layers = h(RectangleGroupIcon)
export const Link = h(HeroLinkIcon)
export const LinkIcon = h(HeroLinkIcon)
export const Loader2 = h(ArrowPathIcon)
export const Lock = h(LockClosedIcon)
export const Map = h(MapIcon)
export const Megaphone = h(MegaphoneIcon)
export const Menu = h(Bars3Icon)
export const Minus = h(MinusIcon)
export const Monitor = h(ComputerDesktopIcon)
export const Moon = h(MoonIcon)
export const Network = h(SignalIcon)
export const OpenInNew = h(ArrowTopRightOnSquareIcon)
export const Pause = h(PauseIcon)
export const People = h(UserGroupIcon)
export const Play = h(PlayIcon)
export const Plus = h(PlusIcon)
export const PlusCircle = h(PlusCircleIcon)
export const Power = h(PowerIcon)
export const Public = h(GlobeAltIcon)
export const Radio = h(SignalIcon)
export const Refresh = h(ArrowPathRoundedSquareIcon)
export const RefreshCcw = h(ArrowPathRoundedSquareIcon)
export const RefreshCw = h(ArrowPathIcon)
export const Save = h(DocumentCheckIcon)
export const Search = h(MagnifyingGlassIcon)
export const Server = h(ServerIcon)
export const Settings = h(CogIcon)
export const Share = h(ShareIcon)
export const Share2 = h(ShareIcon)
export const Shield = h(ShieldCheckIcon)
export const ShieldAlert = h(ShieldExclamationIcon)
export const ShieldCheck = h(ShieldCheckIcon)
export const Sparkles = h(SparklesIcon)
export const Square = h(Squares2X2Icon)
export const Stop = h(StopIcon)
export const Sun = h(SunIcon)
export const Sync = h(ArrowPathIcon)
export const Timer = h(ClockIcon)
export const Trash2 = h(TrashIcon)
export const Users = h(UserGroupIcon)
export const Verified = h(CheckBadgeIcon)
export const Wifi = h(WifiIcon)
export const Workflow = h(ArrowPathRoundedSquareIcon)
export const X = h(XMarkIcon)
export const XCircle = h(XCircleIcon)
export const Zap = h(BoltIcon)
