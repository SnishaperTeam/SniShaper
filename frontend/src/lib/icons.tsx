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
  ArrowDownIcon, ArrowRightIcon,
  ArrowPathIcon, ArrowPathRoundedSquareIcon, ArrowTopRightOnSquareIcon,
  ArrowDownTrayIcon,
  BellAlertIcon, BoltIcon,
  CheckBadgeIcon, CheckCircleIcon, ChevronDownIcon, ChevronUpIcon,
  ClockIcon, CodeBracketIcon, ComputerDesktopIcon,
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
  ShareIcon, ShieldCheckIcon, ShieldExclamationIcon,
  SignalIcon, SparklesIcon, Squares2X2Icon, StopIcon, SunIcon,
  TrashIcon,
  UserGroupIcon,
  WifiIcon,
  XMarkIcon, XCircleIcon,
} from '@heroicons/react/24/outline'

import {
  CheckBadgeIcon as CheckSquareSolid,
} from '@heroicons/react/24/solid'

export const Activity = h(RectangleGroupIcon)
export const AddCircle = h(PlusCircleIcon)
export const AlertCircle = h(ExclamationCircleIcon)
export const Anchor = h(LifebuoyIcon)
export const Antenna = h(SignalIcon)
export const ArrowDown = h(ArrowDownIcon)
export const ArrowForward = h(ArrowRightIcon)
export const BellRing = h(BellAlertIcon)
export const Bolt = h(BoltIcon)
export const Cancel = h(XCircleIcon)
export const CheckCircle = h(CheckCircleIcon)
export const CheckSquare = h(CheckSquareSolid)
export const ChevronDown = h(ChevronDownIcon)
export const ChevronsUp = h(ChevronUpIcon)
export const ChevronUp = h(ChevronUpIcon)
export const Code2 = h(CodeBracketIcon)
export const Cpu = h(CpuChipIcon)
export const Delete = h(TrashIcon)
export const Download = h(ArrowDownTrayIcon)
export const Edit = h(PencilIcon)
export const ExternalLink = h(ArrowTopRightOnSquareIcon)
export const FileText = h(DocumentTextIcon)
export const Filter = h(FunnelIcon)
export const FolderOpen = h(FolderOpenIcon)
export const Globe = h(GlobeAltIcon)
export const Heart = h(HeartIcon)
export const History = h(ClockIcon)
export const Info = h(InformationCircleIcon)
export const Languages = h(LanguageIcon)
export const Link = h(HeroLinkIcon)
export const Lock = h(LockClosedIcon)
export const Map = h(MapIcon)
export const Megaphone = h(MegaphoneIcon)
export const Minus = h(MinusIcon)
export const Monitor = h(ComputerDesktopIcon)
export const Moon = h(MoonIcon)
export const Pause = h(PauseIcon)
export const Play = h(PlayIcon)
export const Plus = h(PlusIcon)
export const Power = h(PowerIcon)
export const RefreshCcw = h(ArrowPathRoundedSquareIcon)
export const RefreshCw = h(ArrowPathIcon)
export const Save = h(DocumentCheckIcon)
export const Search = h(MagnifyingGlassIcon)
export const Settings = h(CogIcon)
export const Share = h(ShareIcon)
export const Shield = h(ShieldCheckIcon)
export const ShieldAlert = h(ShieldExclamationIcon)
export const Sparkles = h(SparklesIcon)
export const Square = h(Squares2X2Icon)
export const Stop = h(StopIcon)
export const Sun = h(SunIcon)
export const Users = h(UserGroupIcon)
export const Wifi = h(WifiIcon)
export const X = h(XMarkIcon)
