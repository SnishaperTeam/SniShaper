import React from 'react'
import { Icon } from '@iconify/react'
import type { IconifyIcon } from '@iconify/react'

// Material design iconography via Iconify (Material Symbols / MDI),
// bundled offline per icon — no runtime API calls.
function m(data: IconifyIcon) {
  return React.forwardRef<SVGSVGElement, any>(({ size, fontSize, className, ...props }, ref) => {
    const numericFontSize = typeof fontSize === 'number' ? fontSize : undefined;
    const s = size || numericFontSize || 24
    return <Icon ref={ref} icon={data} width={s} height={s} className={className} {...props} />
  })
}

import activity from '@iconify-icons/material-symbols/dashboard'
import add from '@iconify-icons/material-symbols/add'
import addCircle from '@iconify-icons/material-symbols/add-circle'
import arrowDownward from '@iconify-icons/material-symbols/arrow-downward'
import arrowForward from '@iconify-icons/material-symbols/arrow-forward'
import autoAwesome from '@iconify-icons/material-symbols/auto-awesome'
import bolt from '@iconify-icons/material-symbols/bolt'
import campaign from '@iconify-icons/material-symbols/campaign'
import cancelIcon from '@iconify-icons/material-symbols/cancel'
import cellTower from '@iconify-icons/material-symbols/cell-tower'
import checkCircle from '@iconify-icons/material-symbols/check-circle'
import close from '@iconify-icons/material-symbols/close'
import code from '@iconify-icons/material-symbols/code'
import computer from '@iconify-icons/material-symbols/computer'
import darkMode from '@iconify-icons/material-symbols/dark-mode'
import deleteIcon from '@iconify-icons/material-symbols/delete'
import description from '@iconify-icons/material-symbols/description'
import download from '@iconify-icons/material-symbols/download'
import edit from '@iconify-icons/material-symbols/edit'
import error from '@iconify-icons/material-symbols/error'
import favorite from '@iconify-icons/material-symbols/favorite'
import filterAlt from '@iconify-icons/material-symbols/filter-alt'
import folderOpen from '@iconify-icons/material-symbols/folder-open'
import gppMaybe from '@iconify-icons/material-symbols/gpp-maybe'
import grid from '@iconify-icons/material-symbols/grid-view'
import groups from '@iconify-icons/material-symbols/groups'
import history from '@iconify-icons/material-symbols/history'
import info from '@iconify-icons/material-symbols/info'
import keyboardArrowDown from '@iconify-icons/material-symbols/keyboard-arrow-down'
import keyboardArrowUp from '@iconify-icons/material-symbols/keyboard-arrow-up'
import lightMode from '@iconify-icons/material-symbols/light-mode'
import link from '@iconify-icons/material-symbols/link'
import lock from '@iconify-icons/material-symbols/lock'
import map from '@iconify-icons/material-symbols/map'
import memory from '@iconify-icons/material-symbols/memory'
import notificationsActive from '@iconify-icons/material-symbols/notifications-active'
import openInNew from '@iconify-icons/material-symbols/open-in-new'
import pause from '@iconify-icons/material-symbols/pause'
import playArrow from '@iconify-icons/material-symbols/play-arrow'
import powerSettingsNew from '@iconify-icons/material-symbols/power-settings-new'
import publicGlobe from '@iconify-icons/material-symbols/public'
import refresh from '@iconify-icons/material-symbols/refresh'
import remove from '@iconify-icons/material-symbols/remove'
import save from '@iconify-icons/material-symbols/save'
import search from '@iconify-icons/material-symbols/search'
import settings from '@iconify-icons/material-symbols/settings'
import share from '@iconify-icons/material-symbols/share'
import stop from '@iconify-icons/material-symbols/stop'
import sync from '@iconify-icons/material-symbols/sync'
import translate from '@iconify-icons/material-symbols/translate'
import verified from '@iconify-icons/material-symbols/verified'
import verifiedUser from '@iconify-icons/material-symbols/verified-user'
import wifi from '@iconify-icons/material-symbols/wifi'
import lifebuoy from '@iconify-icons/mdi/lifebuoy'

export const Activity = m(activity)
export const AddCircle = m(addCircle)
export const AlertCircle = m(error)
export const Anchor = m(lifebuoy)
export const Antenna = m(cellTower)
export const ArrowDown = m(arrowDownward)
export const ArrowForward = m(arrowForward)
export const BellRing = m(notificationsActive)
export const Bolt = m(bolt)
export const Cancel = m(cancelIcon)
export const CheckCircle = m(checkCircle)
export const CheckSquare = m(verified)
export const ChevronDown = m(keyboardArrowDown)
export const ChevronsUp = m(keyboardArrowUp)
export const ChevronUp = m(keyboardArrowUp)
export const Code2 = m(code)
export const Cpu = m(memory)
export const Delete = m(deleteIcon)
export const Download = m(download)
export const Edit = m(edit)
export const ExternalLink = m(openInNew)
export const FileText = m(description)
export const Filter = m(filterAlt)
export const FolderOpen = m(folderOpen)
export const Globe = m(publicGlobe)
export const Heart = m(favorite)
export const History = m(history)
export const Info = m(info)
export const Languages = m(translate)
export const Link = m(link)
export const Lock = m(lock)
export const Map = m(map)
export const Megaphone = m(campaign)
export const Minus = m(remove)
export const Monitor = m(computer)
export const Moon = m(darkMode)
export const Pause = m(pause)
export const Play = m(playArrow)
export const Plus = m(add)
export const Power = m(powerSettingsNew)
export const RefreshCcw = m(sync)
export const RefreshCw = m(refresh)
export const Save = m(save)
export const Search = m(search)
export const Settings = m(settings)
export const Share = m(share)
export const Shield = m(verifiedUser)
export const ShieldAlert = m(gppMaybe)
export const Sparkles = m(autoAwesome)
export const Square = m(grid)
export const Stop = m(stop)
export const Sun = m(lightMode)
export const Users = m(groups)
export const Wifi = m(wifi)
export const X = m(close)
