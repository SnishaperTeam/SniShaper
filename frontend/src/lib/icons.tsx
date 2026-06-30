import { Icon as IconifyIcon } from '@iconify/react';
import React from 'react';

function L(name: string) {
  return React.forwardRef<SVGSVGElement, any>(({ size, ...props }, ref) => (
    <IconifyIcon ref={ref} icon={`lucide:${name}`} width={size || 24} {...props} />
  ));
}

function LN(name: string) {
  return L(name);
}

export const Activity = L('activity');
export const AlertCircle = L('alert-circle');
export const Anchor = L('anchor');
export const Antenna = L('antenna');
export const ArrowDown = L('arrow-down');
export const ArrowDownRight = L('arrow-down-right');
export const ArrowRight = L('arrow-right');
export const ArrowUp = L('arrow-up');
export const ArrowUpRight = L('arrow-up-right');
export const BellRing = L('bell-ring');
export const CheckCircle2 = L('check-circle-2');
export const CheckSquare = L('check-square');
export const ChevronDown = L('chevron-down');
export const ChevronsUp = L('chevrons-up');
export const ChevronUp = L('chevron-up');
export const Cloud = L('cloud');
export const CloudLightning = L('cloud-lightning');
export const Code2 = L('code-2');
export const Cpu = L('cpu');
export const Download = L('download');
export const Edit3 = L('edit-3');
export const ExternalLink = L('external-link');
export const FileText = L('file-text');
export const Filter = L('filter');
export const FolderOpen = L('folder-open');
export const GitBranch = L('git-branch');
export const Globe = L('globe');
export const Heart = L('heart');
export const History = L('history');
export const Info = L('info');
export const Languages = L('languages');
export const LayoutDashboard = L('layout-dashboard');
export const Link = L('link');
export const LinkIcon = L('link');
export const Loader2 = L('loader-2');
export const Lock = L('lock');
export const Map = L('map');
export const Megaphone = L('megaphone');
export const Menu = L('menu');
export const Minus = L('minus');
export const Monitor = L('monitor');
export const Moon = L('moon');
export const Network = L('network');
export const Pause = L('pause');
export const Play = L('play');
export const Plus = L('plus');
export const PlusCircle = L('plus-circle');
export const Power = L('power');
export const Radio = L('radio');
export const RefreshCcw = L('refresh-ccw');
export const RefreshCw = L('refresh-cw');
export const Save = L('save');
export const Search = L('search');
export const Server = L('server');
export const Settings = L('settings');
export const Share2 = L('share-2');
export const Shield = L('shield');
export const ShieldAlert = L('shield-alert');
export const ShieldCheck = L('shield-check');
export const Sparkles = L('sparkles');
export const Square = L('square');
export const Sun = L('sun');
export const Timer = L('timer');
export const Trash2 = L('trash-2');
export const Users = L('users');
export const Wifi = L('wifi');
export const Workflow = L('workflow');
export const X = L('x');
export const XCircle = L('x-circle');
export const Zap = L('zap');
