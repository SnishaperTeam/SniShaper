# MITM Real-SNI Verify Report

- Generated at: 2026-03-29T19:55:12+08:00
- Config: `rules/config.json`
- Timeout: `8s`
- Total checked domains: `221`
- compatible: `119`
- incompatible: `81`
- skipped_ech: `21`

## compatible (119)

- `a.pixiv.org` addr=`210.140.139.183:443` peerCN=`pixiv.org` alpn=`h2`
  SANs: `pixiv.org, *.pixiv.org`
- `d.pixiv.org` group=`a.pixiv.org` addr=`210.140.139.183:443` peerCN=`pixiv.org` alpn=`h2`
  SANs: `pixiv.org, *.pixiv.org`
- `abs-0.twimg.com` addr=`146.75.48.159:443` sni=`abs-0-twimg-com` peerCN=`*.twimg.com` alpn=`h2`
  SANs: `*.twimg.com, cdn.syndication.twimg.com, platform.twitter.com, twimg.com`
- `account-api.proton.me` addr=`185.70.42.20:443` sni=`account-api-proton-me.mapped` peerCN=`proton.me` alpn=`h2`
  SANs: `*.pr.tn, *.proton.me, *.storage.proton.me, pr.tn, proton.me`
- `account.proton.me` addr=`185.70.42.36:443` sni=`account-proton-me.mapped` peerCN=`proton.me` alpn=`h2`
  SANs: `*.pr.tn, *.proton.me, *.storage.proton.me, pr.tn, proton.me`
- `alive.github.com` addr=`140.82.112.26:443` sni=`alive-github-com.mapped` peerCN=`*.github.com` alpn=`http/1.1`
  SANs: `*.github.com, github.com`
- `android.com` group=`android` addr=`47.102.115.14:443` sni=`g.cn` peerCN=`*.google.com` alpn=`h2`
  SANs: `*.google.com, *.appengine.google.com, *.bdn.dev, *.origin-test.bdn.dev, *.cloud.google.com, *.crowdsource.google.com, ... +131 more`
- `bbc.com` group=`api.bbc.com` addr=`146.75.36.81:443` sni=`bh.lk` peerCN=`www.bbc.com` alpn=`h2`
  SANs: `www.bbc.com, www.bbc.co.uk, www.bbcrussian.com, bbc.co.uk, bbcrussian.com, session.bbc.co.uk, ... +15 more`
- `api.e-hentai.org` addr=`5.79.104.110:443` sni=`api-e-hentai-org` peerCN=`e-hentai.org` alpn=`h2`
  SANs: `*.e-hentai.org, e-hentai.org`
- `api.github.com` addr=`20.205.243.168:443` sni=`api-github-com.mapped` peerCN=`*.github.com` alpn=`h2`
  SANs: `*.github.com, github.com`
- `lfs.github.com` group=`api.github.com` addr=`20.205.243.168:443` sni=`api-github-com.mapped` peerCN=`*.github.com` alpn=`h2`
  SANs: `*.github.com, github.com`
- `api.mega.co.nz` addr=`66.203.125.15:443` sni=`api-mega-co-nz` peerCN=`*.api.mega.co.nz` alpn=`http/1.1`
  SANs: `*.api.mega.co.nz, api.mega.co.nz`
- `apkmirror.com` addr=`104.17.67.215:443` sni=`apkmirror-com` peerCN=`apkmirror.com` alpn=`h2`
  SANs: `apkmirror.com, *.apkmirror.com`
- `archiveofourown.org` addr=`104.20.9.2:443` sni=`archiveofourown-org` peerCN=`archiveofourown.org` alpn=`h2`
  SANs: `archiveofourown.org, *.archiveofourown.org`
- `assets.twitch.tv` addr=`151.101.194.167:443` peerCN=`twitch.tv` alpn=`http/1.1`
  SANs: `twitch.tv, *.twitch.tv`
- `panels.twitch.tv` group=`assets.twitch.tv` addr=`151.101.194.167:443` peerCN=`twitch.tv` alpn=`http/1.1`
  SANs: `twitch.tv, *.twitch.tv`
- `passport.twitch.tv` group=`assets.twitch.tv` addr=`151.101.194.167:443` peerCN=`twitch.tv` alpn=`http/1.1`
  SANs: `twitch.tv, *.twitch.tv`
- `beacon.dropbox.com` addr=`162.125.40.1:443` sni=`beacon-dropbox-com.mapped` peerCN=`api-notify.dropbox.com` alpn=`h2`
  SANs: `api-notify.dropbox.com, api-notify0.dropbox.com, beacon.dropbox.com, bolt.dropbox.com, notify.dropboxapi.com, thunder.dropbox.com`
- `bolt.dropbox.com` addr=`162.125.40.1:443` sni=`bolt-dropbox-com.mapped` peerCN=`api-notify.dropbox.com` alpn=`h2`
  SANs: `api-notify.dropbox.com, api-notify0.dropbox.com, beacon.dropbox.com, bolt.dropbox.com, notify.dropboxapi.com, thunder.dropbox.com`
- `facebook.com` group=`business.whatsapp.com` addr=`157.240.22.169:443` sni=`facebook-com` peerCN=`*.facebook.com` alpn=`h2`
  SANs: `*.facebook.com, *.facebook.net, *.fbcdn.net, *.fbsbx.com, *.m.facebook.com, *.messenger.com, ... +5 more`
- `calendar.proton.me` addr=`185.70.42.39:443` sni=`calendar-proton-me.mapped` peerCN=`proton.me` alpn=`h2`
  SANs: `*.pr.tn, *.proton.me, *.storage.proton.me, pr.tn, proton.me`
- `cdn.jsdelivr.net` addr=`151.101.65.229:443` sni=`cdn-jsdelivr-net.mapped` peerCN=`jsdelivr.net` alpn=`h2`
  SANs: `jsdelivr.net, *.jsdelivr.net`
- `jsdelivr.net` group=`cdn.jsdelivr.net` addr=`151.101.65.229:443` sni=`cdn-jsdelivr-net.mapped` peerCN=`jsdelivr.net` alpn=`h2`
  SANs: `jsdelivr.net, *.jsdelivr.net`
- `codeload.github.com` addr=`20.205.243.165:443` sni=`codeload-github-com.mapped` peerCN=`*.github.com` alpn=`h2`
  SANs: `*.github.com, github.com`
- `collector.github.com` group=`community.github.com` addr=`140.82.112.17:443` sni=`community-github-com.mapped` peerCN=`*.github.com` alpn=`h2`
  SANs: `*.github.com, github.com`
- `community.github.com` addr=`140.82.112.17:443` sni=`community-github-com.mapped` peerCN=`*.github.com` alpn=`h2`
  SANs: `*.github.com, github.com`
- `copilot.github.com` group=`community.github.com` addr=`140.82.112.17:443` sni=`community-github-com.mapped` peerCN=`*.github.com` alpn=`h2`
  SANs: `*.github.com, github.com`
- `redirect.github.com` group=`community.github.com` addr=`140.82.112.17:443` sni=`community-github-com.mapped` peerCN=`*.github.com` alpn=`h2`
  SANs: `*.github.com, github.com`
- `services.github.com` group=`community.github.com` addr=`140.82.112.17:443` sni=`community-github-com.mapped` peerCN=`*.github.com` alpn=`h2`
  SANs: `*.github.com, github.com`
- `d.dropbox.com` addr=`162.125.6.20:443` sni=`d-dropbox-com.mapped` peerCN=`*.dropbox.com` alpn=`h2`
  SANs: `*.dropbox.com, dropbox.com`
- `discord.com` addr=`162.159.128.233:443` sni=`discord-com` peerCN=`discord.com` alpn=`h2`
  SANs: `discord.com, *.discord.com`
- `discord.gg` addr=`162.159.136.234:443` sni=`discord-gg` peerCN=`discord.gg` alpn=`h2`
  SANs: `discord.gg, *.discord.gg`
- `discordapp.com` addr=`162.159.135.233:443` sni=`discordapp-com` peerCN=`discordapp.com` alpn=`h2`
  SANs: `discordapp.com, *.discordapp.com`
- `discordapp.net` addr=`162.159.129.232:443` sni=`media-discordapp-net` peerCN=`discordapp.net` alpn=`h2`
  SANs: `discordapp.net, media.discordapp.net`
- `dl.dropboxusercontent.com` addr=`162.125.1.15:443` sni=`dl-dropboxusercontent-com` peerCN=`*.dl-au.dropboxusercontent.com` alpn=`h2`
  SANs: `*.dl-au.dropboxusercontent.com, *.dl-eu.dropboxusercontent.com, *.dl-jp.dropboxusercontent.com, *.dl-uk.dropboxusercontent.com, *.dl.dropboxusercontent.com, dl-au.dropbox.com, ... +24 more`
- `githubassets.com` group=`docs.github.com` addr=`185.199.109.154:443` sni=`docs-github-com.mapped` peerCN=`*.githubassets.com` alpn=`h2`
  SANs: `*.githubassets.com, githubassets.com`
- `drive.proton.me` addr=`185.70.42.40:443` sni=`drive-proton-me.mapped` peerCN=`proton.me` alpn=`h2`
  SANs: `*.pr.tn, *.proton.me, *.storage.proton.me, pr.tn, proton.me`
- `dropbox.com` addr=`162.125.248.18:443` sni=`dropbox-com.mapped` peerCN=`*.dropbox.com` alpn=`h2`
  SANs: `*.dropbox.com, dropbox.com`
- `duckduckgo.com` addr=`20.43.161.105:443` sni=`duckduckgo-com` peerCN=`*.duckduckgo.com` alpn=`h2`
  SANs: `*.duckduckgo.com, duckduckgo.com`
- `ehgt.org` addr=`89.39.106.43:443` sni=`ehgt-org` peerCN=`ehgt.org` alpn=`h2`
  SANs: `ehgt.org, www.ehgt.org`
- `et.nytimes.com` addr=`146.75.117.164:443` sni=`et-nytimes-com` peerCN=`nytimes.com` alpn=`h2`
  SANs: `nytimes.com, www.homedelivery.nytimes.com, *.api.dev.nytimes.com, *.api.nytimes.com, *.api.stg.nytimes.com, *.blogs.nytimes.com, ... +23 more`
- `nyt.com` group=`et.nytimes.com` addr=`146.75.117.164:443` sni=`nyt-com` peerCN=`nytimes.com` alpn=`h2`
  SANs: `nytimes.com, www.homedelivery.nytimes.com, *.api.dev.nytimes.com, *.api.nytimes.com, *.api.stg.nytimes.com, *.blogs.nytimes.com, ... +23 more`
- `nytimes.com` group=`et.nytimes.com` addr=`146.75.117.164:443` sni=`nytimes-com` peerCN=`nytimes.com` alpn=`h2`
  SANs: `nytimes.com, www.homedelivery.nytimes.com, *.api.dev.nytimes.com, *.api.nytimes.com, *.api.stg.nytimes.com, *.blogs.nytimes.com, ... +23 more`
- `etsy.com` addr=`151.101.193.224:443` sni=`etsy-com` peerCN=`*.etsystatic.com` alpn=`h2`
  SANs: `*.etsystatic.com, api-origin.etsy.com, api.etsy.com, m.etsy.com, openapi.etsy.com, www.etsy.com, ... +2 more`
- `external-content.duckduckgo.com` addr=`52.250.30.213:443` sni=`external-content-duckduckgo-com` peerCN=`*.duckduckgo.com` alpn=`h2`
  SANs: `*.duckduckgo.com, duckduckgo.com`
- `f-droid.org` addr=`37.218.243.72:443` sni=`f-droid-org` peerCN=`f-droid.org` alpn=`h2`
  SANs: `f-droid.com, f-droid.org, fdroid.com, fdroid.org, www.f-droid.com, www.f-droid.org, ... +2 more`
- `flickr.com` addr=`13.33.142.102:443` sni=`flickr-com` peerCN=`flickr.com`
  SANs: `flickr.com, *.flickr.com, flic.kr`
- `forum.f-droid.org` addr=`37.218.242.53:443` sni=`forum-f-droid-org` peerCN=`forum.f-droid.org` alpn=`h2`
  SANs: `forum.f-droid.org`
- `github.com` group=`gist.github.com` addr=`20.205.243.166:443` sni=`gist-github-com.mapped` peerCN=`github.com` alpn=`h2`
  SANs: `github.com, www.github.com`
- `www.github.com` group=`gist.github.com` addr=`20.205.243.166:443` sni=`gist-github-com.mapped` peerCN=`github.com` alpn=`h2`
  SANs: `github.com, www.github.com`
- `gql.twitch.tv` addr=`151.101.194.167:443` peerCN=`twitch.tv` alpn=`http/1.1`
  SANs: `twitch.tv, *.twitch.tv`
- `m.twitch.tv` group=`gql.twitch.tv` addr=`151.101.194.167:443` peerCN=`twitch.tv` alpn=`http/1.1`
  SANs: `twitch.tv, *.twitch.tv`
- `twitch.tv` group=`gql.twitch.tv` addr=`151.101.194.167:443` peerCN=`twitch.tv` alpn=`http/1.1`
  SANs: `twitch.tv, *.twitch.tv`
- `www.twitch.tv` group=`gql.twitch.tv` addr=`151.101.194.167:443` peerCN=`twitch.tv` alpn=`http/1.1`
  SANs: `twitch.tv, *.twitch.tv`
- `gravatar.com` addr=`192.0.80.240:443` sni=`gravatar-com` peerCN=`gravatar.com` alpn=`h2`
  SANs: `*.gravatar.com, gravatar.com`
- `greasyfork.org` addr=`96.126.98.220:443` sni=`g.cn` peerCN=`greasyfork.org` alpn=`h2`
  SANs: `api.cn-greasyfork.org, api.greasyfork.org, api.sleazyfork.org, cn-greasyfork.org, greasyfork.org, sleazyfork.org, ... +6 more`
- `i.pximg.net` addr=`210.140.139.135:443` peerCN=`pximg.net` alpn=`h2`
  SANs: `pximg.net, *.pximg.net`
- `img-works.pximg.net` group=`i.pximg.net` addr=`210.140.139.135:443` peerCN=`pximg.net` alpn=`h2`
  SANs: `pximg.net, *.pximg.net`
- `s.pximg.net` group=`i.pximg.net` addr=`210.140.139.135:443` peerCN=`pximg.net` alpn=`h2`
  SANs: `pximg.net, *.pximg.net`
- `imgur.com` addr=`199.232.196.193:443` sni=`imgur-com` peerCN=`*.imgur.com` alpn=`h2`
  SANs: `*.imgur.com, imgur.com`
- `cdninstagram.com` group=`instagram` addr=`157.240.236.174:443` sni=`fa.aq` peerCN=`*.instagram.com` alpn=`h2`
  SANs: `*.instagram.com, *.cdninstagram.com, *.igsonar.com, cdninstagram.com, igsonar.com, instagram.com`
- `instagram.com` group=`instagram` addr=`157.240.236.174:443` sni=`fa.aq` peerCN=`*.instagram.com` alpn=`h2`
  SANs: `*.instagram.com, *.cdninstagram.com, *.igsonar.com, cdninstagram.com, igsonar.com, instagram.com`
- `static.cdninstagram.com` group=`instagram` addr=`157.240.236.174:443` sni=`fa.aq` peerCN=`*.instagram.com` alpn=`h2`
  SANs: `*.instagram.com, *.cdninstagram.com, *.igsonar.com, cdninstagram.com, igsonar.com, instagram.com`
- `lc-event.pixiv.net` addr=`210.140.139.185:443` peerCN=`pixiv.net` alpn=`h2`
  SANs: `pixiv.net, *.pixiv.net, pixiv.me, public-api.secure.pixiv.net, oauth.secure.pixiv.net, www.pixivision.net, ... +2 more`
- `mail.proton.me` addr=`185.70.42.37:443` sni=`mail-proton-me.mapped` peerCN=`proton.me` alpn=`h2`
  SANs: `*.pr.tn, *.proton.me, *.storage.proton.me, pr.tn, proton.me`
- `mega.nz` addr=`31.216.144.5:443` sni=`mega-nz` peerCN=`mega.nz` alpn=`http/1.1`
  SANs: `mega.nz, www.mega.nz`
- `ms.ok.ru` addr=`5.61.23.11:443` sni=`ms-ok-ru` peerCN=`*.ok.ru` alpn=`h2`
  SANs: `*.ok.ru, ms.ok.ru, *.dating.ok.ru, *.m.ok.ru, *.ms.ok.ru, *.mscu.ok.ru, ... +25 more`
- `ok.ru` group=`ms.ok.ru` addr=`5.61.23.11:443` sni=`ok-ru` peerCN=`*.ok.ru` alpn=`h2`
  SANs: `*.ok.ru, ms.ok.ru, *.dating.ok.ru, *.m.ok.ru, *.ms.ok.ru, *.mscu.ok.ru, ... +25 more`
- `pass.proton.me` addr=`185.70.42.63:443` sni=`pass-proton-me.mapped` peerCN=`proton.me` alpn=`h2`
  SANs: `*.pr.tn, *.proton.me, *.storage.proton.me, pr.tn, proton.me`
- `patreon.com` addr=`104.16.25.14:443` sni=`patreon-com` peerCN=`patreon.com` alpn=`h2`
  SANs: `patreon.com, *.patreon.com`
- `patreonusercontent.com` addr=`104.18.70.106:443` sni=`patreonusercontent-com` peerCN=`patreonusercontent.com` alpn=`h2`
  SANs: `patreonusercontent.com, *.patreonusercontent.com`
- `pinimg.com` addr=`151.101.0.84:443` sni=`pinimg-com` peerCN=`*.pinterest.com` alpn=`h2`
  SANs: `*.pinterest.com, *.pinimg.com, *.pinterest.info, *.pinterest.engineering, *.pinterestmail.com, *.pinterest.at, ... +90 more`
- `pinterest.com` group=`pinimg.com` addr=`151.101.0.84:443` sni=`pinterest-com` peerCN=`*.pinterest.com` alpn=`h2`
  SANs: `*.pinterest.com, *.pinimg.com, *.pinterest.info, *.pinterest.engineering, *.pinterestmail.com, *.pinterest.at, ... +90 more`
- `app.pixiv.net` group=`pixiv.net` addr=`210.140.139.152:443` peerCN=`pixiv.net` alpn=`h2`
  SANs: `pixiv.net, *.pixiv.net, pixiv.me, public-api.secure.pixiv.net, oauth.secure.pixiv.net, www.pixivision.net, ... +2 more`
- `oauth.secure.pixiv.net` group=`pixiv.net` addr=`210.140.139.152:443` peerCN=`pixiv.net` alpn=`h2`
  SANs: `pixiv.net, *.pixiv.net, pixiv.me, public-api.secure.pixiv.net, oauth.secure.pixiv.net, www.pixivision.net, ... +2 more`
- `pixiv.net` addr=`210.140.139.152:443` peerCN=`pixiv.net` alpn=`h2`
  SANs: `pixiv.net, *.pixiv.net, pixiv.me, public-api.secure.pixiv.net, oauth.secure.pixiv.net, www.pixivision.net, ... +2 more`
- `www.pixiv.net` group=`pixiv.net` addr=`210.140.139.152:443` peerCN=`pixiv.net` alpn=`h2`
  SANs: `pixiv.net, *.pixiv.net, pixiv.me, public-api.secure.pixiv.net, oauth.secure.pixiv.net, www.pixivision.net, ... +2 more`
- `platform.twitter.com` addr=`146.75.120.157:443` sni=`platform-twitter-com` peerCN=`*.twimg.com` alpn=`h2`
  SANs: `*.twimg.com, cdn.syndication.twimg.com, platform.twitter.com, twimg.com`
- `previews.dropbox.com` addr=`162.125.83.16:443` sni=`previews-dropbox-com.mapped` peerCN=`*.previews.dropboxusercontent.com` alpn=`h2`
  SANs: `*.previews.dropboxusercontent.com, api-content-photos.dropbox.com, photos-1.dropbox.com, photos-2.dropbox.com, photos-3.dropbox.com, photos-4.dropbox.com, ... +5 more`
- `proton.me` addr=`185.70.42.45:443` sni=`proton-me.mapped` peerCN=`proton.me` alpn=`h2`
  SANs: `*.pr.tn, *.proton.me, *.storage.proton.me, pr.tn, proton.me`
- `pximg.net` addr=`210.140.139.135:443` peerCN=`pximg.net` alpn=`h2`
  SANs: `pximg.net, *.pximg.net`
- `reddit.com` group=`redd.it` addr=`146.75.33.140:443` sni=`reddit-com` peerCN=`*.reddit.com` alpn=`h2`
  SANs: `*.reddit.com, reddit.com`
- `rfi.fr` addr=`118.214.247.61:443` sni=`rfi-fr` peerCN=`www.rfi.fr` alpn=`http/1.1`
  SANs: `www.rfi.fr, alexa.voice-api.rfi.fr, amp.rfi.fr, api2.rfi.fr, apis.fle.rfi.fr, apis.rfi.fr, ... +61 more`
- `rumble.com` addr=`205.220.231.24:443` sni=`rumble-com` peerCN=`*.rumble.com` alpn=`h2`
  SANs: `*.rumble.com, rumble.com`
- `rutube.ru` addr=`178.248.233.148:443` sni=`rutub.io` peerCN=`*.rutube.ru` alpn=`h2`
  SANs: `*.rutube.ru, rutube.ru`
- `www.rutube.ru` group=`rutube.ru` addr=`178.248.233.148:443` sni=`rutub.io` peerCN=`*.rutube.ru` alpn=`h2`
  SANs: `*.rutube.ru, rutube.ru`
- `steamcommunity.com` group=`steam社区` addr=`184.24.82.93:443` sni=`steamcom.mapped` peerCN=`store.steampowered.com` alpn=`http/1.1`
  SANs: `store.steampowered.com, api.steampowered.com, help.steampowered.com, login.steampowered.com, partner.steamgames.com, partner.steampowered.com, ... +3 more`
- `telegram.org` group=`t.me` addr=`136.244.94.246:443` sni=`g.cn` peerCN=`*.telegram.org` alpn=`h2`
  SANs: `*.telegram.org, telegram.org`
- `thetvdb.com` addr=`13.35.222.88:443` sni=`thetvdb-com` peerCN=`*.thetvdb.com`
  SANs: `*.thetvdb.com, *.beta.thetvdb.com, thetvdb.com, *.prod.thetvdb.com`
- `thunder.dropbox.com` addr=`162.125.40.1:443` sni=`thunder-dropbox-com.mapped` peerCN=`api-notify.dropbox.com` alpn=`h2`
  SANs: `api-notify.dropbox.com, api-notify0.dropbox.com, beacon.dropbox.com, bolt.dropbox.com, notify.dropboxapi.com, thunder.dropbox.com`
- `twimg.com` addr=`146.75.72.157:443` sni=`twimg-com` peerCN=`*.twimg.com` alpn=`h2`
  SANs: `*.twimg.com, cdn.syndication.twimg.com, platform.twitter.com, twimg.com`
- `upload.wikimedia.org` addr=`208.80.153.240:443` sni=`upload-wikipedia-org` peerCN=`upload.wikimedia.org` alpn=`h2`
  SANs: `maps.wikimedia.org, upload.wikimedia.org`
- `video.pscp.tv` addr=`146.75.34.164:443` sni=`video-pscp-tv` peerCN=`*.video.pscp.tv` alpn=`h2`
  SANs: `*.video.pscp.tv, video.pscp.tv`
- `whatsapp.com` addr=`157.240.225.60:443` sni=`whatsapp-com` peerCN=`*.whatsapp.net` alpn=`h2`
  SANs: `*.whatsapp.net, *.cdn.whatsapp.net, *.snr.whatsapp.net, *.whatsapp.com, wa.me, whatsapp.com, ... +1 more`
- `whatsapp.net` group=`whatsapp.com` addr=`157.240.225.60:443` sni=`whatsapp-net` peerCN=`*.whatsapp.net` alpn=`h2`
  SANs: `*.whatsapp.net, *.cdn.whatsapp.net, *.snr.whatsapp.net, *.whatsapp.com, wa.me, whatsapp.com, ... +1 more`
- `m.mediawiki.org` group=`wikipedia` addr=`208.80.153.224:443` sni=`wikipedia-org` peerCN=`*.wikipedia.org` alpn=`h2`
  SANs: `*.m.mediawiki.org, *.m.wikibooks.org, *.m.wikidata.org, *.m.wikimedia.org, *.m.wikinews.org, *.m.wikipedia.org, ... +35 more`
- `m.wikibooks.org` group=`wikipedia` addr=`208.80.153.224:443` sni=`wikipedia-org` peerCN=`*.wikipedia.org` alpn=`h2`
  SANs: `*.m.mediawiki.org, *.m.wikibooks.org, *.m.wikidata.org, *.m.wikimedia.org, *.m.wikinews.org, *.m.wikipedia.org, ... +35 more`
- `m.wikidata.org` group=`wikipedia` addr=`208.80.153.224:443` sni=`wikipedia-org` peerCN=`*.wikipedia.org` alpn=`h2`
  SANs: `*.m.mediawiki.org, *.m.wikibooks.org, *.m.wikidata.org, *.m.wikimedia.org, *.m.wikinews.org, *.m.wikipedia.org, ... +35 more`
- `m.wikifunctions.org` group=`wikipedia` addr=`208.80.153.224:443` sni=`wikipedia-org` peerCN=`*.wikipedia.org` alpn=`h2`
  SANs: `*.m.mediawiki.org, *.m.wikibooks.org, *.m.wikidata.org, *.m.wikimedia.org, *.m.wikinews.org, *.m.wikipedia.org, ... +35 more`
- `m.wikimedia.org` group=`wikipedia` addr=`208.80.153.224:443` sni=`wikipedia-org` peerCN=`*.wikipedia.org` alpn=`h2`
  SANs: `*.m.mediawiki.org, *.m.wikibooks.org, *.m.wikidata.org, *.m.wikimedia.org, *.m.wikinews.org, *.m.wikipedia.org, ... +35 more`
- `m.wikinews.org` group=`wikipedia` addr=`208.80.153.224:443` sni=`wikipedia-org` peerCN=`*.wikipedia.org` alpn=`h2`
  SANs: `*.m.mediawiki.org, *.m.wikibooks.org, *.m.wikidata.org, *.m.wikimedia.org, *.m.wikinews.org, *.m.wikipedia.org, ... +35 more`
- `m.wikipedia.org` group=`wikipedia` addr=`208.80.153.224:443` sni=`wikipedia-org` peerCN=`*.wikipedia.org` alpn=`h2`
  SANs: `*.m.mediawiki.org, *.m.wikibooks.org, *.m.wikidata.org, *.m.wikimedia.org, *.m.wikinews.org, *.m.wikipedia.org, ... +35 more`
- `m.wikiquote.org` group=`wikipedia` addr=`208.80.153.224:443` sni=`wikipedia-org` peerCN=`*.wikipedia.org` alpn=`h2`
  SANs: `*.m.mediawiki.org, *.m.wikibooks.org, *.m.wikidata.org, *.m.wikimedia.org, *.m.wikinews.org, *.m.wikipedia.org, ... +35 more`
- `m.wikiversity.org` group=`wikipedia` addr=`208.80.153.224:443` sni=`wikipedia-org` peerCN=`*.wikipedia.org` alpn=`h2`
  SANs: `*.m.mediawiki.org, *.m.wikibooks.org, *.m.wikidata.org, *.m.wikimedia.org, *.m.wikinews.org, *.m.wikipedia.org, ... +35 more`
- `m.wikivoyage.org` group=`wikipedia` addr=`208.80.153.224:443` sni=`wikipedia-org` peerCN=`*.wikipedia.org` alpn=`h2`
  SANs: `*.m.mediawiki.org, *.m.wikibooks.org, *.m.wikidata.org, *.m.wikimedia.org, *.m.wikinews.org, *.m.wikipedia.org, ... +35 more`
- `m.wiktionary.org` group=`wikipedia` addr=`208.80.153.224:443` sni=`wikipedia-org` peerCN=`*.wikipedia.org` alpn=`h2`
  SANs: `*.m.mediawiki.org, *.m.wikibooks.org, *.m.wikidata.org, *.m.wikimedia.org, *.m.wikinews.org, *.m.wikipedia.org, ... +35 more`
- `mediawiki.org` group=`wikipedia` addr=`208.80.153.224:443` sni=`wikipedia-org` peerCN=`*.wikipedia.org` alpn=`h2`
  SANs: `*.m.mediawiki.org, *.m.wikibooks.org, *.m.wikidata.org, *.m.wikimedia.org, *.m.wikinews.org, *.m.wikipedia.org, ... +35 more`
- `wikibooks.org` group=`wikipedia` addr=`208.80.153.224:443` sni=`wikipedia-org` peerCN=`*.wikipedia.org` alpn=`h2`
  SANs: `*.m.mediawiki.org, *.m.wikibooks.org, *.m.wikidata.org, *.m.wikimedia.org, *.m.wikinews.org, *.m.wikipedia.org, ... +35 more`
- `wikidata.org` group=`wikipedia` addr=`208.80.153.224:443` sni=`wikipedia-org` peerCN=`*.wikipedia.org` alpn=`h2`
  SANs: `*.m.mediawiki.org, *.m.wikibooks.org, *.m.wikidata.org, *.m.wikimedia.org, *.m.wikinews.org, *.m.wikipedia.org, ... +35 more`
- `wikifunctions.org` group=`wikipedia` addr=`208.80.153.224:443` sni=`wikipedia-org` peerCN=`*.wikipedia.org` alpn=`h2`
  SANs: `*.m.mediawiki.org, *.m.wikibooks.org, *.m.wikidata.org, *.m.wikimedia.org, *.m.wikinews.org, *.m.wikipedia.org, ... +35 more`
- `wikimedia.org` group=`wikipedia` addr=`208.80.153.224:443` sni=`wikipedia-org` peerCN=`*.wikipedia.org` alpn=`h2`
  SANs: `*.m.mediawiki.org, *.m.wikibooks.org, *.m.wikidata.org, *.m.wikimedia.org, *.m.wikinews.org, *.m.wikipedia.org, ... +35 more`
- `wikinews.org` group=`wikipedia` addr=`208.80.153.224:443` sni=`wikipedia-org` peerCN=`*.wikipedia.org` alpn=`h2`
  SANs: `*.m.mediawiki.org, *.m.wikibooks.org, *.m.wikidata.org, *.m.wikimedia.org, *.m.wikinews.org, *.m.wikipedia.org, ... +35 more`
- `wikipedia.org` group=`wikipedia` addr=`208.80.153.224:443` sni=`wikipedia-org` peerCN=`*.wikipedia.org` alpn=`h2`
  SANs: `*.m.mediawiki.org, *.m.wikibooks.org, *.m.wikidata.org, *.m.wikimedia.org, *.m.wikinews.org, *.m.wikipedia.org, ... +35 more`
- `wikiquote.org` group=`wikipedia` addr=`208.80.153.224:443` sni=`wikipedia-org` peerCN=`*.wikipedia.org` alpn=`h2`
  SANs: `*.m.mediawiki.org, *.m.wikibooks.org, *.m.wikidata.org, *.m.wikimedia.org, *.m.wikinews.org, *.m.wikipedia.org, ... +35 more`
- `wikisource.org` group=`wikipedia` addr=`208.80.153.224:443` sni=`wikipedia-org` peerCN=`*.wikipedia.org` alpn=`h2`
  SANs: `*.m.mediawiki.org, *.m.wikibooks.org, *.m.wikidata.org, *.m.wikimedia.org, *.m.wikinews.org, *.m.wikipedia.org, ... +35 more`
- `wikiversity.org` group=`wikipedia` addr=`208.80.153.224:443` sni=`wikipedia-org` peerCN=`*.wikipedia.org` alpn=`h2`
  SANs: `*.m.mediawiki.org, *.m.wikibooks.org, *.m.wikidata.org, *.m.wikimedia.org, *.m.wikinews.org, *.m.wikipedia.org, ... +35 more`
- `wikivoyage.org` group=`wikipedia` addr=`208.80.153.224:443` sni=`wikipedia-org` peerCN=`*.wikipedia.org` alpn=`h2`
  SANs: `*.m.mediawiki.org, *.m.wikibooks.org, *.m.wikidata.org, *.m.wikimedia.org, *.m.wikinews.org, *.m.wikipedia.org, ... +35 more`
- `wiktionary.org` group=`wikipedia` addr=`208.80.153.224:443` sni=`wikipedia-org` peerCN=`*.wikipedia.org` alpn=`h2`
  SANs: `*.m.mediawiki.org, *.m.wikibooks.org, *.m.wikidata.org, *.m.wikimedia.org, *.m.wikinews.org, *.m.wikipedia.org, ... +35 more`
- `www.dropbox.com` addr=`162.125.80.18:443` sni=`www-dropbox-com.mapped` peerCN=`*.dropbox.com` alpn=`h2`
  SANs: `*.dropbox.com, dropbox.com`

## incompatible (81)

- `amazon.co.jp` sni=`amazon.com`
  reason: `18.66.145.15:443: x509: certificate is valid for www.amazon.com, yp.amazon.com, konrad-test.amazon.com, p-yo-www-amazon-com-kalias.amazon.com, mp3recs.amazon.com, p-nt-www-amazon-com-kalias.amazon.com, origin-www.amazon.com, buybox.amazon.com, uedata.amazon.com, us.amazon.com, yellowpages.amazon.com, home.amazon.com, www.m.amazon.com, iphone.amazon.com, www.amzn.com, huddles.amazon.com, amazon.com, corporate.amazon.com, shop.business.amazon.com, p-y3-www-amazon-com-kalias.amazon.com, buckeye-retail-website.amazon.com, www.cdn.amazon.com, test-www.amazon.com, amzn.com, not amazon.co.jp`
- `gstatic.com` group=`android` sni=`g.cn`
  reason: `47.102.115.14:443: x509: certificate is valid for 137 names, but none matched gstatic.com`
- `api.bbc.co.uk` sni=`bh.lk`
  reason: `23.77.21.232:443: x509: certificate is valid for *.cdn.cyberarena.at, cdn.cyberarena.at, not api.bbc.co.uk`
- `api.bbci.co.uk` group=`api.bbc.co.uk` sni=`bh.lk`
  reason: `23.77.21.232:443: x509: certificate is valid for *.cdn.cyberarena.at, cdn.cyberarena.at, not api.bbci.co.uk`
- `bbc.co.uk` group=`api.bbc.co.uk` sni=`bh.lk`
  reason: `23.77.21.232:443: x509: certificate is valid for *.cdn.cyberarena.at, cdn.cyberarena.at, not bbc.co.uk`
- `bbci.co.uk` group=`api.bbc.co.uk` sni=`bh.lk`
  reason: `23.77.21.232:443: x509: certificate is valid for *.cdn.cyberarena.at, cdn.cyberarena.at, not bbci.co.uk`
- `files.bbci.co.uk` group=`api.bbc.co.uk` sni=`bh.lk`
  reason: `23.77.21.232:443: x509: certificate is valid for *.cdn.cyberarena.at, cdn.cyberarena.at, not files.bbci.co.uk`
- `live.bbc.co.uk` group=`api.bbc.co.uk` sni=`bh.lk`
  reason: `23.77.21.232:443: x509: certificate is valid for *.cdn.cyberarena.at, cdn.cyberarena.at, not live.bbc.co.uk`
- `static.bbci.co.uk` group=`api.bbc.co.uk` sni=`bh.lk`
  reason: `23.77.21.232:443: x509: certificate is valid for *.cdn.cyberarena.at, cdn.cyberarena.at, not static.bbci.co.uk`
- `api.bbc.com` sni=`bh.lk`
  reason: `146.75.36.81:443: x509: certificate is valid for www.bbc.com, www.bbc.co.uk, www.bbcrussian.com, bbc.co.uk, bbcrussian.com, session.bbc.co.uk, search.bbc.co.uk, open.live.bbc.co.uk, news.bbc.co.uk, newsimg.bbc.co.uk, wwwnews.live.bbc.co.uk, cdnedge.bbc.co.uk, newsrss.bbc.co.uk, newsvote.bbc.co.uk, playlists.bbc.co.uk, r.bbci.co.uk, node1.bbcimg.co.uk, news.bbcimg.co.uk, account.bbc.com, session.bbc.com, bbc.com, not api.bbc.com`
- `assets.dropbox.com` sni=`assets-dropbox-com.mapped`
  reason: `216.137.39.27:443: remote error: tls: handshake failure`
- `irc-ws.chat.twitch.tv` group=`assets.twitch.tv`
  reason: `151.101.194.167:443: x509: certificate is valid for twitch.tv, *.twitch.tv, not irc-ws.chat.twitch.tv`
- `audiomack.com`
  reason: `3.167.200.113:443: x509: certificate is valid for cloudfront.net, *.cloudfront.net, not audiomack.com`
- `prod.us-east-1.aws.audiomack.com` group=`audiomack.com`
  reason: `3.167.200.113:443: x509: certificate is valid for cloudfront.net, *.cloudfront.net, not prod.us-east-1.aws.audiomack.com`
- `business.whatsapp.com` sni=`business-whatsapp-com`
  reason: `157.240.22.169:443: x509: certificate is valid for *.facebook.com, *.facebook.net, *.fbcdn.net, *.fbsbx.com, *.m.facebook.com, *.messenger.com, *.xx.fbcdn.net, *.xy.fbcdn.net, *.xz.fbcdn.net, facebook.com, messenger.com, not business.whatsapp.com`
- `fbcdn.net` group=`business.whatsapp.com` sni=`fbcdn-net`
  reason: `157.240.22.169:443: x509: certificate is valid for *.facebook.com, *.facebook.net, *.fbcdn.net, *.fbsbx.com, *.m.facebook.com, *.messenger.com, *.xx.fbcdn.net, *.xy.fbcdn.net, *.xz.fbcdn.net, facebook.com, messenger.com, not fbcdn.net`
- `cdn1.cdn-telegram.org` sni=`g.cn`
  reason: `34.111.15.3:443: x509: certificate has expired or is not yet valid: `
- `cdn4.cdn-telegram.org` sni=`g.cn`
  reason: `34.111.35.152:443: x509: certificate has expired or is not yet valid: `
- `cdn5.cdn-telegram.org` sni=`g.cn`
  reason: `34.111.108.175:443: x509: certificate has expired or is not yet valid: `
- `central.github.com` sni=`centr.map`
  reason: `140.82.112.21:443: x509: certificate is valid for *.actions.githubusercontent.com, actions.githubusercontent.com, not central.github.com; 140.82.112.22:443: x509: certificate is valid for *.actions.githubusercontent.com, actions.githubusercontent.com, not central.github.com; 140.82.113.21:443: x509: certificate is valid for *.actions.githubusercontent.com, actions.githubusercontent.com, not central.github.com; 140.82.113.22:443: x509: certificate is valid for *.actions.githubusercontent.com, actions.githubusercontent.com, not central.github.com; 140.82.114.21:443: x509: certificate is valid for *.actions.githubusercontent.com, actions.githubusercontent.com, not central.github.com; 140.82.114.22:443: x509: certificate is valid for *.actions.githubusercontent.com, actions.githubusercontent.com, not central.github.com`
- `classroom.github.com` group=`central.github.com` sni=`centr.map`
  reason: `140.82.112.21:443: x509: certificate is valid for *.actions.githubusercontent.com, actions.githubusercontent.com, not classroom.github.com; 140.82.112.22:443: x509: certificate is valid for *.actions.githubusercontent.com, actions.githubusercontent.com, not classroom.github.com; 140.82.113.21:443: x509: certificate is valid for *.actions.githubusercontent.com, actions.githubusercontent.com, not classroom.github.com; 140.82.113.22:443: x509: certificate is valid for *.actions.githubusercontent.com, actions.githubusercontent.com, not classroom.github.com; 140.82.114.21:443: x509: certificate is valid for *.actions.githubusercontent.com, actions.githubusercontent.com, not classroom.github.com; 140.82.114.22:443: x509: certificate is valid for *.actions.githubusercontent.com, actions.githubusercontent.com, not classroom.github.com`
- `collector.github.com` group=`central.github.com` sni=`centr.map`
  reason: `140.82.112.21:443: x509: certificate is valid for *.actions.githubusercontent.com, actions.githubusercontent.com, not collector.github.com; 140.82.112.22:443: x509: certificate is valid for *.actions.githubusercontent.com, actions.githubusercontent.com, not collector.github.com; 140.82.113.21:443: context deadline exceeded; 140.82.113.22:443: x509: certificate is valid for *.actions.githubusercontent.com, actions.githubusercontent.com, not collector.github.com; 140.82.114.21:443: x509: certificate is valid for *.actions.githubusercontent.com, actions.githubusercontent.com, not collector.github.com; 140.82.114.22:443: x509: certificate is valid for *.actions.githubusercontent.com, actions.githubusercontent.com, not collector.github.com`
- `education.github.com` group=`central.github.com` sni=`centr.map`
  reason: `140.82.112.21:443: x509: certificate is valid for *.actions.githubusercontent.com, actions.githubusercontent.com, not education.github.com; 140.82.112.22:443: x509: certificate is valid for *.actions.githubusercontent.com, actions.githubusercontent.com, not education.github.com; 140.82.113.21:443: x509: certificate is valid for *.actions.githubusercontent.com, actions.githubusercontent.com, not education.github.com; 140.82.113.22:443: x509: certificate is valid for *.actions.githubusercontent.com, actions.githubusercontent.com, not education.github.com; 140.82.114.21:443: x509: certificate is valid for *.actions.githubusercontent.com, actions.githubusercontent.com, not education.github.com; 140.82.114.22:443: x509: certificate is valid for *.actions.githubusercontent.com, actions.githubusercontent.com, not education.github.com`
- `enterprise.github.com` group=`central.github.com` sni=`centr.map`
  reason: `140.82.112.21:443: x509: certificate is valid for *.actions.githubusercontent.com, actions.githubusercontent.com, not enterprise.github.com; 140.82.112.22:443: x509: certificate is valid for *.actions.githubusercontent.com, actions.githubusercontent.com, not enterprise.github.com; 140.82.113.21:443: x509: certificate is valid for *.actions.githubusercontent.com, actions.githubusercontent.com, not enterprise.github.com; 140.82.113.22:443: x509: certificate is valid for *.actions.githubusercontent.com, actions.githubusercontent.com, not enterprise.github.com; 140.82.114.21:443: x509: certificate is valid for *.actions.githubusercontent.com, actions.githubusercontent.com, not enterprise.github.com; 140.82.114.22:443: x509: certificate is valid for *.actions.githubusercontent.com, actions.githubusercontent.com, not enterprise.github.com`
- `previews.dropboxusercontent.com` group=`dl.dropboxusercontent.com` sni=`dl-dropboxusercontent-com`
  reason: `162.125.1.15:443: x509: certificate is valid for *.dl-au.dropboxusercontent.com, *.dl-eu.dropboxusercontent.com, *.dl-jp.dropboxusercontent.com, *.dl-uk.dropboxusercontent.com, *.dl.dropboxusercontent.com, dl-au.dropbox.com, dl-au.dropboxusercontent.com, dl-eu.dropbox.com, dl-eu.dropboxusercontent.com, dl-jp.dropbox.com, dl-jp.dropboxusercontent.com, dl-uk.dropbox.com, dl-uk.dropboxusercontent.com, dl-web-au.dropbox.com, dl-web-eu.dropbox.com, dl-web-jp.dropbox.com, dl-web-uk.dropbox.com, dl-web.dropbox.com, dl.dropbox.com, dl.dropboxusercontent.com, files-au.dropbox.com, files-eu.dropbox.com, files-jp.dropbox.com, files-uk.dropbox.com, files.dropbox.com, showbox-au.dropbox.com, showbox-eu.dropbox.com, showbox-jp.dropbox.com, showbox-uk.dropbox.com, showbox.dropbox.com, not previews.dropboxusercontent.com; 162.125.2.15:443: x509: certificate is valid for *.dl-au.dropboxusercontent.com, *.dl-eu.dropboxusercontent.com, *.dl-jp.dropboxusercontent.com, *.dl-uk.dropboxusercontent.com, *.dl.dropboxusercontent.com, dl-au.dropbox.com, dl-au.dropboxusercontent.com, dl-eu.dropbox.com, dl-eu.dropboxusercontent.com, dl-jp.dropbox.com, dl-jp.dropboxusercontent.com, dl-uk.dropbox.com, dl-uk.dropboxusercontent.com, dl-web-au.dropbox.com, dl-web-eu.dropbox.com, dl-web-jp.dropbox.com, dl-web-uk.dropbox.com, dl-web.dropbox.com, dl.dropbox.com, dl.dropboxusercontent.com, files-au.dropbox.com, files-eu.dropbox.com, files-jp.dropbox.com, files-uk.dropbox.com, files.dropbox.com, showbox-au.dropbox.com, showbox-eu.dropbox.com, showbox-jp.dropbox.com, showbox-uk.dropbox.com, showbox.dropbox.com, not previews.dropboxusercontent.com; 162.125.13.15:443: x509: certificate is valid for *.dl-au.dropboxusercontent.com, *.dl-eu.dropboxusercontent.com, *.dl-jp.dropboxusercontent.com, *.dl-uk.dropboxusercontent.com, *.dl.dropboxusercontent.com, dl-au.dropbox.com, dl-au.dropboxusercontent.com, dl-eu.dropbox.com, dl-eu.dropboxusercontent.com, dl-jp.dropbox.com, dl-jp.dropboxusercontent.com, dl-uk.dropbox.com, dl-uk.dropboxusercontent.com, dl-web-au.dropbox.com, dl-web-eu.dropbox.com, dl-web-jp.dropbox.com, dl-web-uk.dropbox.com, dl-web.dropbox.com, dl.dropbox.com, dl.dropboxusercontent.com, files-au.dropbox.com, files-eu.dropbox.com, files-jp.dropbox.com, files-uk.dropbox.com, files.dropbox.com, showbox-au.dropbox.com, showbox-eu.dropbox.com, showbox-jp.dropbox.com, showbox-uk.dropbox.com, showbox.dropbox.com, not previews.dropboxusercontent.com; 162.125.8.15:443: x509: certificate is valid for *.dl-au.dropboxusercontent.com, *.dl-eu.dropboxusercontent.com, *.dl-jp.dropboxusercontent.com, *.dl-uk.dropboxusercontent.com, *.dl.dropboxusercontent.com, dl-au.dropbox.com, dl-au.dropboxusercontent.com, dl-eu.dropbox.com, dl-eu.dropboxusercontent.com, dl-jp.dropbox.com, dl-jp.dropboxusercontent.com, dl-uk.dropbox.com, dl-uk.dropboxusercontent.com, dl-web-au.dropbox.com, dl-web-eu.dropbox.com, dl-web-jp.dropbox.com, dl-web-uk.dropbox.com, dl-web.dropbox.com, dl.dropbox.com, dl.dropboxusercontent.com, files-au.dropbox.com, files-eu.dropbox.com, files-jp.dropbox.com, files-uk.dropbox.com, files.dropbox.com, showbox-au.dropbox.com, showbox-eu.dropbox.com, showbox-jp.dropbox.com, showbox-uk.dropbox.com, showbox.dropbox.com, not previews.dropboxusercontent.com; 162.125.69.15:443: x509: certificate is valid for *.dl-au.dropboxusercontent.com, *.dl-eu.dropboxusercontent.com, *.dl-jp.dropboxusercontent.com, *.dl-uk.dropboxusercontent.com, *.dl.dropboxusercontent.com, dl-au.dropbox.com, dl-au.dropboxusercontent.com, dl-eu.dropbox.com, dl-eu.dropboxusercontent.com, dl-jp.dropbox.com, dl-jp.dropboxusercontent.com, dl-uk.dropbox.com, dl-uk.dropboxusercontent.com, dl-web-au.dropbox.com, dl-web-eu.dropbox.com, dl-web-jp.dropbox.com, dl-web-uk.dropbox.com, dl-web.dropbox.com, dl.dropbox.com, dl.dropboxusercontent.com, files-au.dropbox.com, files-eu.dropbox.com, files-jp.dropbox.com, files-uk.dropbox.com, files.dropbox.com, showbox-au.dropbox.com, showbox-eu.dropbox.com, showbox-jp.dropbox.com, showbox-uk.dropbox.com, showbox.dropbox.com, not previews.dropboxusercontent.com; 162.125.64.15:443: x509: certificate is valid for *.dl-au.dropboxusercontent.com, *.dl-eu.dropboxusercontent.com, *.dl-jp.dropboxusercontent.com, *.dl-uk.dropboxusercontent.com, *.dl.dropboxusercontent.com, dl-au.dropbox.com, dl-au.dropboxusercontent.com, dl-eu.dropbox.com, dl-eu.dropboxusercontent.com, dl-jp.dropbox.com, dl-jp.dropboxusercontent.com, dl-uk.dropbox.com, dl-uk.dropboxusercontent.com, dl-web-au.dropbox.com, dl-web-eu.dropbox.com, dl-web-jp.dropbox.com, dl-web-uk.dropbox.com, dl-web.dropbox.com, dl.dropbox.com, dl.dropboxusercontent.com, files-au.dropbox.com, files-eu.dropbox.com, files-jp.dropbox.com, files-uk.dropbox.com, files.dropbox.com, showbox-au.dropbox.com, showbox-eu.dropbox.com, showbox-jp.dropbox.com, showbox-uk.dropbox.com, showbox.dropbox.com, not previews.dropboxusercontent.com; 162.125.68.15:443: x509: certificate is valid for *.dl-au.dropboxusercontent.com, *.dl-eu.dropboxusercontent.com, *.dl-jp.dropboxusercontent.com, *.dl-uk.dropboxusercontent.com, *.dl.dropboxusercontent.com, dl-au.dropbox.com, dl-au.dropboxusercontent.com, dl-eu.dropbox.com, dl-eu.dropboxusercontent.com, dl-jp.dropbox.com, dl-jp.dropboxusercontent.com, dl-uk.dropbox.com, dl-uk.dropboxusercontent.com, dl-web-au.dropbox.com, dl-web-eu.dropbox.com, dl-web-jp.dropbox.com, dl-web-uk.dropbox.com, dl-web.dropbox.com, dl.dropbox.com, dl.dropboxusercontent.com, files-au.dropbox.com, files-eu.dropbox.com, files-jp.dropbox.com, files-uk.dropbox.com, files.dropbox.com, showbox-au.dropbox.com, showbox-eu.dropbox.com, showbox-jp.dropbox.com, showbox-uk.dropbox.com, showbox.dropbox.com, not previews.dropboxusercontent.com; 162.125.71.15:443: x509: certificate is valid for *.dl-au.dropboxusercontent.com, *.dl-eu.dropboxusercontent.com, *.dl-jp.dropboxusercontent.com, *.dl-uk.dropboxusercontent.com, *.dl.dropboxusercontent.com, dl-au.dropbox.com, dl-au.dropboxusercontent.com, dl-eu.dropbox.com, dl-eu.dropboxusercontent.com, dl-jp.dropbox.com, dl-jp.dropboxusercontent.com, dl-uk.dropbox.com, dl-uk.dropboxusercontent.com, dl-web-au.dropbox.com, dl-web-eu.dropbox.com, dl-web-jp.dropbox.com, dl-web-uk.dropbox.com, dl-web.dropbox.com, dl.dropbox.com, dl.dropboxusercontent.com, files-au.dropbox.com, files-eu.dropbox.com, files-jp.dropbox.com, files-uk.dropbox.com, files.dropbox.com, showbox-au.dropbox.com, showbox-eu.dropbox.com, showbox-jp.dropbox.com, showbox-uk.dropbox.com, showbox.dropbox.com, not previews.dropboxusercontent.com; 162.125.72.15:443: x509: certificate is valid for *.dl-au.dropboxusercontent.com, *.dl-eu.dropboxusercontent.com, *.dl-jp.dropboxusercontent.com, *.dl-uk.dropboxusercontent.com, *.dl.dropboxusercontent.com, dl-au.dropbox.com, dl-au.dropboxusercontent.com, dl-eu.dropbox.com, dl-eu.dropboxusercontent.com, dl-jp.dropbox.com, dl-jp.dropboxusercontent.com, dl-uk.dropbox.com, dl-uk.dropboxusercontent.com, dl-web-au.dropbox.com, dl-web-eu.dropbox.com, dl-web-jp.dropbox.com, dl-web-uk.dropbox.com, dl-web.dropbox.com, dl.dropbox.com, dl.dropboxusercontent.com, files-au.dropbox.com, files-eu.dropbox.com, files-jp.dropbox.com, files-uk.dropbox.com, files.dropbox.com, showbox-au.dropbox.com, showbox-eu.dropbox.com, showbox-jp.dropbox.com, showbox-uk.dropbox.com, showbox.dropbox.com, not previews.dropboxusercontent.com; 162.125.65.15:443: x509: certificate is valid for *.dl-au.dropboxusercontent.com, *.dl-eu.dropboxusercontent.com, *.dl-jp.dropboxusercontent.com, *.dl-uk.dropboxusercontent.com, *.dl.dropboxusercontent.com, dl-au.dropbox.com, dl-au.dropboxusercontent.com, dl-eu.dropbox.com, dl-eu.dropboxusercontent.com, dl-jp.dropbox.com, dl-jp.dropboxusercontent.com, dl-uk.dropbox.com, dl-uk.dropboxusercontent.com, dl-web-au.dropbox.com, dl-web-eu.dropbox.com, dl-web-jp.dropbox.com, dl-web-uk.dropbox.com, dl-web.dropbox.com, dl.dropbox.com, dl.dropboxusercontent.com, files-au.dropbox.com, files-eu.dropbox.com, files-jp.dropbox.com, files-uk.dropbox.com, files.dropbox.com, showbox-au.dropbox.com, showbox-eu.dropbox.com, showbox-jp.dropbox.com, showbox-uk.dropbox.com, showbox.dropbox.com, not previews.dropboxusercontent.com; 162.125.81.15:443: x509: certificate is valid for *.dl-au.dropboxusercontent.com, *.dl-eu.dropboxusercontent.com, *.dl-jp.dropboxusercontent.com, *.dl-uk.dropboxusercontent.com, *.dl.dropboxusercontent.com, dl-au.dropbox.com, dl-au.dropboxusercontent.com, dl-eu.dropbox.com, dl-eu.dropboxusercontent.com, dl-jp.dropbox.com, dl-jp.dropboxusercontent.com, dl-uk.dropbox.com, dl-uk.dropboxusercontent.com, dl-web-au.dropbox.com, dl-web-eu.dropbox.com, dl-web-jp.dropbox.com, dl-web-uk.dropbox.com, dl-web.dropbox.com, dl.dropbox.com, dl.dropboxusercontent.com, files-au.dropbox.com, files-eu.dropbox.com, files-jp.dropbox.com, files-uk.dropbox.com, files.dropbox.com, showbox-au.dropbox.com, showbox-eu.dropbox.com, showbox-jp.dropbox.com, showbox-uk.dropbox.com, showbox.dropbox.com, not previews.dropboxusercontent.com; 162.125.80.15:443: x509: certificate is valid for *.dl-au.dropboxusercontent.com, *.dl-eu.dropboxusercontent.com, *.dl-jp.dropboxusercontent.com, *.dl-uk.dropboxusercontent.com, *.dl.dropboxusercontent.com, dl-au.dropbox.com, dl-au.dropboxusercontent.com, dl-eu.dropbox.com, dl-eu.dropboxusercontent.com, dl-jp.dropbox.com, dl-jp.dropboxusercontent.com, dl-uk.dropbox.com, dl-uk.dropboxusercontent.com, dl-web-au.dropbox.com, dl-web-eu.dropbox.com, dl-web-jp.dropbox.com, dl-web-uk.dropbox.com, dl-web.dropbox.com, dl.dropbox.com, dl.dropboxusercontent.com, files-au.dropbox.com, files-eu.dropbox.com, files-jp.dropbox.com, files-uk.dropbox.com, files.dropbox.com, showbox-au.dropbox.com, showbox-eu.dropbox.com, showbox-jp.dropbox.com, showbox-uk.dropbox.com, showbox.dropbox.com, not previews.dropboxusercontent.com; 162.125.85.15:443: x509: certificate is valid for *.dl-au.dropboxusercontent.com, *.dl-eu.dropboxusercontent.com, *.dl-jp.dropboxusercontent.com, *.dl-uk.dropboxusercontent.com, *.dl.dropboxusercontent.com, dl-au.dropbox.com, dl-au.dropboxusercontent.com, dl-eu.dropbox.com, dl-eu.dropboxusercontent.com, dl-jp.dropbox.com, dl-jp.dropboxusercontent.com, dl-uk.dropbox.com, dl-uk.dropboxusercontent.com, dl-web-au.dropbox.com, dl-web-eu.dropbox.com, dl-web-jp.dropbox.com, dl-web-uk.dropbox.com, dl-web.dropbox.com, dl.dropbox.com, dl.dropboxusercontent.com, files-au.dropbox.com, files-eu.dropbox.com, files-jp.dropbox.com, files-uk.dropbox.com, files.dropbox.com, showbox-au.dropbox.com, showbox-eu.dropbox.com, showbox-jp.dropbox.com, showbox-uk.dropbox.com, showbox.dropbox.com, not previews.dropboxusercontent.com`
- `docs.github.com` sni=`docs-github-com.mapped`
  reason: `185.199.109.154:443: x509: certificate is valid for *.githubassets.com, githubassets.com, not docs.github.com; 185.199.108.154:443: x509: certificate is valid for *.githubassets.com, githubassets.com, not docs.github.com; 185.199.110.154:443: x509: certificate is valid for *.githubassets.com, githubassets.com, not docs.github.com; 185.199.111.154:443: x509: certificate is valid for *.githubassets.com, githubassets.com, not docs.github.com`
- `github.io` group=`docs.github.com` sni=`docs-github-com.mapped`
  reason: `185.199.109.154:443: x509: certificate is valid for *.githubassets.com, githubassets.com, not github.io; 185.199.108.154:443: x509: certificate is valid for *.githubassets.com, githubassets.com, not github.io; 185.199.110.154:443: x509: certificate is valid for *.githubassets.com, githubassets.com, not github.io; 185.199.111.154:443: x509: certificate is valid for *.githubassets.com, githubassets.com, not github.io`
- `githubusercontent.com` group=`docs.github.com` sni=`docs-github-com.mapped`
  reason: `185.199.109.154:443: x509: certificate is valid for *.githubassets.com, githubassets.com, not githubusercontent.com; 185.199.108.154:443: x509: certificate is valid for *.githubassets.com, githubassets.com, not githubusercontent.com; 185.199.110.154:443: x509: certificate is valid for *.githubassets.com, githubassets.com, not githubusercontent.com; 185.199.111.154:443: x509: certificate is valid for *.githubassets.com, githubassets.com, not githubusercontent.com`
- `download.tails.net` sni=`download-tails-net`
  reason: `204.13.164.63:443: x509: certificate is valid for deb.tails.boum.org, not download.tails.net`
- `gemini.google.com` sni=`g.cn`
  reason: `47.102.115.14:443: EOF`
- `gist.github.com` sni=`gist-github-com.mapped`
  reason: `20.205.243.166:443: x509: certificate is valid for github.com, www.github.com, not gist.github.com; 20.27.177.113:443: x509: certificate is valid for github.com, www.github.com, not gist.github.com; 20.200.245.247:443: x509: certificate is valid for github.com, www.github.com, not gist.github.com`
- `*.googleapis.com` group=`google.com` sni=`g.cn`
  reason: `34.49.133.3:443: x509: certificate has expired or is not yet valid: `
- `antigravity.google` group=`google.com` sni=`g.cn`
  reason: `34.49.133.3:443: x509: certificate has expired or is not yet valid: `
- `blogger.com` group=`google.com` sni=`g.cn`
  reason: `34.49.133.3:443: x509: certificate has expired or is not yet valid: `
- `google.com` sni=`g.cn`
  reason: `34.49.133.3:443: x509: certificate has expired or is not yet valid: `
- `google.com.*` group=`google.com` sni=`g.cn`
  reason: `34.49.133.3:443: x509: certificate has expired or is not yet valid: `
- `googleapis.com` group=`google.com` sni=`g.cn`
  reason: `34.49.133.3:443: x509: certificate has expired or is not yet valid: `
- `googleusercontent.com` group=`google.com` sni=`g.cn`
  reason: `34.49.133.3:443: x509: certificate has expired or is not yet valid: `
- `lh3.googleusercontent.com` group=`google.com` sni=`g.cn`
  reason: `34.49.133.3:443: x509: certificate has expired or is not yet valid: `
- `googleapis.com` group=`googleapi` sni=`g.cn`
  reason: `180.163.150.34:443: x509: certificate is valid for 137 names, but none matched googleapis.com`
- `ggpht.com` group=`gstatic.com` sni=`g.cn`
  reason: `142.250.20.90:443: dial tcp 142.250.20.90:443: i/o timeout`
- `gmail.com` group=`gstatic.com` sni=`g.cn`
  reason: `142.250.20.90:443: dial tcp 142.250.20.90:443: i/o timeout`
- `youtubei.googleapis.com` group=`gstatic.com` sni=`g.cn`
  reason: `142.250.20.90:443: dial tcp 142.250.20.90:443: i/o timeout`
- `cdn-avatars.huggingface.co` group=`huggingface`
  reason: `3.167.200.113:443: x509: certificate is valid for cloudfront.net, *.cloudfront.net, not cdn-avatars.huggingface.co`
- `cdn-uploads.huggingface.co` group=`huggingface`
  reason: `3.167.200.113:443: x509: certificate is valid for cloudfront.net, *.cloudfront.net, not cdn-uploads.huggingface.co`
- `discuss.huggingface.co` group=`huggingface`
  reason: `3.167.200.113:443: x509: certificate is valid for cloudfront.net, *.cloudfront.net, not discuss.huggingface.co`
- `huggingface.co` group=`huggingface`
  reason: `3.167.200.113:443: x509: certificate is valid for cloudfront.net, *.cloudfront.net, not huggingface.co`
- `status.huggingface.co` group=`huggingface`
  reason: `3.167.200.113:443: x509: certificate is valid for cloudfront.net, *.cloudfront.net, not status.huggingface.co`
- `identity.flickr.com` sni=`identity-flickr-com`
  reason: `3.209.240.130:443: x509: certificate is valid for *.dev.madsense.io, not identity.flickr.com`
- `images.prismic.io` sni=`images-prismic-io`
  reason: `151.101.78.208:443: x509: certificate is valid for *.imgix.com, *.imgix.net, imgix.com, imgix.net, not images.prismic.io`
- `ig.me` group=`instagram` sni=`fa.aq`
  reason: `157.240.236.174:443: x509: certificate is valid for *.instagram.com, *.cdninstagram.com, *.igsonar.com, cdninstagram.com, igsonar.com, instagram.com, not ig.me`
- `instagr.am` group=`instagram` sni=`fa.aq`
  reason: `157.240.236.174:443: x509: certificate is valid for *.instagram.com, *.cdninstagram.com, *.igsonar.com, cdninstagram.com, igsonar.com, instagram.com, not instagr.am`
- `threads.com` group=`instagram` sni=`fa.aq`
  reason: `157.240.236.174:443: x509: certificate is valid for *.instagram.com, *.cdninstagram.com, *.igsonar.com, cdninstagram.com, igsonar.com, instagram.com, not threads.com`
- `threads.net` group=`instagram` sni=`fa.aq`
  reason: `157.240.236.174:443: x509: certificate is valid for *.instagram.com, *.cdninstagram.com, *.igsonar.com, cdninstagram.com, igsonar.com, instagram.com, not threads.net`
- `mega.io` sni=`mega-io`
  reason: `66.203.127.11:443: x509: certificate is valid for *.static.mega.co.nz, static.mega.co.nz, not mega.io`
- `nyaa.si` sni=`nyaa-si`
  reason: `186.2.163.20:443: x509: certificate signed by unknown authority`
- `objects-origin.githubusercontent.com` sni=`objects-origin-githubusercontent-com.mapped`
  reason: `140.82.113.22:443: x509: certificate is valid for *.actions.githubusercontent.com, actions.githubusercontent.com, not objects-origin.githubusercontent.com`
- `okx.com` sni=`okx-com`
  reason: `8.212.101.92:443: x509: certificate is not valid for any names, but wanted to match okx.com`
- `onedrive.live.com` sni=`onedrive-live-com`
  reason: `13.107.42.13:443: read tcp 10.89.157.200:5864->13.107.42.13:443: wsarecv: An existing connection was forcibly closed by the remote host.`
- `pixiv.pximg.net` group=`pixiv.net`
  reason: `210.140.139.152:443: x509: certificate is valid for pixiv.net, *.pixiv.net, pixiv.me, public-api.secure.pixiv.net, oauth.secure.pixiv.net, www.pixivision.net, fanbox.cc, *.fanbox.cc, not pixiv.pximg.net`
- `quora.com` sni=`fs.quoracdn.net`
  reason: `18.233.40.74:443: x509: certificate is valid for *.us-east-1.cloudsearch.amazonaws.com, not quora.com`
- `tch.quora.com` group=`quora.com` sni=`fs.quoracdn.net`
  reason: `18.233.40.74:443: x509: certificate is valid for *.us-east-1.cloudsearch.amazonaws.com, not tch.quora.com`
- `www.quora.com` group=`quora.com` sni=`fs.quoracdn.net`
  reason: `18.233.40.74:443: x509: certificate is valid for *.us-east-1.cloudsearch.amazonaws.com, not www.quora.com`
- `www.tch.quora.com` group=`quora.com` sni=`fs.quoracdn.net`
  reason: `18.233.40.74:443: x509: certificate is valid for *.us-east-1.cloudsearch.amazonaws.com, not www.tch.quora.com`
- `redd.it` sni=`redd-it`
  reason: `146.75.33.140:443: x509: certificate is valid for *.reddit.com, reddit.com, not redd.it`
- `redditmedia.com` group=`redd.it` sni=`redditmedia-com`
  reason: `146.75.33.140:443: x509: certificate is valid for *.reddit.com, reddit.com, not redditmedia.com`
- `redditstatic.com` group=`redd.it` sni=`redditstatic-com`
  reason: `146.75.33.140:443: x509: certificate is valid for *.reddit.com, reddit.com, not redditstatic.com`
- `community.steamstatic.com` group=`steam资源` sni=`comm.mapped`
  reason: `199.232.211.52:443: x509: certificate is valid for t.sni-820-default.ssl.fastly.net, not community.steamstatic.com`
- `sukebei.nyaa.si` sni=`sukebei-nyaa-si`
  reason: `198.251.89.38:443: x509: certificate signed by unknown authority`
- `t.me` sni=`g.cn`
  reason: `136.244.94.246:443: x509: certificate is valid for *.telegram.org, telegram.org, not t.me`
- `telegram.me` group=`t.me` sni=`g.cn`
  reason: `136.244.94.246:443: x509: certificate is valid for *.telegram.org, telegram.org, not telegram.me`
- `telesco.pe` group=`t.me` sni=`g.cn`
  reason: `136.244.94.246:443: x509: certificate is valid for *.telegram.org, telegram.org, not telesco.pe`
- `tg.dev` group=`t.me` sni=`g.cn`
  reason: `136.244.94.246:443: x509: certificate is valid for *.telegram.org, telegram.org, not tg.dev`
- `tails.net` sni=`tails-net`
  reason: `94.142.244.34:443: x509: certificate is valid for mta-sts.tails.net, not tails.net`
- `upld.e-hentai.org` sni=`upld-e-hentai-org`
  reason: `95.211.208.236:443: dial tcp 95.211.208.236:443: connectex: No connection could be made because the target machine actively refused it.`
- `upload.e-hentai.org` group=`upld.e-hentai.org` sni=`upload-e-hentai-org`
  reason: `95.211.208.236:443: write tcp 10.89.157.200:5893->95.211.208.236:443: wsasend: An existing connection was forcibly closed by the remote host.`
- `uploads.github.com` sni=`uploads-github-com.mapped`
  reason: `20.207.73.81:443: x509: certificate is valid for *.githubusercontent.com, githubusercontent.com, not uploads.github.com`
- `vercel.app` sni=`vercel.com`
  reason: `64.29.17.193:443: x509: certificate is valid for *.vercel.com, vercel.com, not vercel.app`
- `xnxx.com` sni=`xm.mapped`
  reason: `152.233.100.3:443: x509: certificate signed by unknown authority`
- `xvideos.com` group=`xvideos` sni=`xv.os`
  reason: `89.222.127.11:443: x509: certificate signed by unknown authority`
- `z-library.sk` group=`zlib sk` sni=`z.lo`
  reason: `94.231.223.5:443: x509: certificate signed by unknown authority`

## skipped_ech (21)

- `assets.chapturist.com`
  reason: `ECH rule skipped; external real-SNI verification is only implemented for non-ECH MITM paths`
- `cloudflare.com`
  reason: `ECH rule skipped; external real-SNI verification is only implemented for non-ECH MITM paths`
- `e-hentai.org`
  reason: `ECH rule skipped; external real-SNI verification is only implemented for non-ECH MITM paths`
- `exhentai.org`
  reason: `ECH rule skipped; external real-SNI verification is only implemented for non-ECH MITM paths`
- `fanbox.cc`
  reason: `ECH rule skipped; external real-SNI verification is only implemented for non-ECH MITM paths`
- `forums.e-hentai.org`
  reason: `ECH rule skipped; external real-SNI verification is only implemented for non-ECH MITM paths`
- `gelbooru.com`
  reason: `ECH rule skipped; external real-SNI verification is only implemented for non-ECH MITM paths`
- `gfw.report`
  reason: `ECH rule skipped; external real-SNI verification is only implemented for non-ECH MITM paths`
- `hcaptcha.com`
  reason: `ECH rule skipped; external real-SNI verification is only implemented for non-ECH MITM paths`
- `hub.docker.com`
  reason: `ECH rule skipped; external real-SNI verification is only implemented for non-ECH MITM paths`
- `itch.io`
  reason: `ECH rule skipped; external real-SNI verification is only implemented for non-ECH MITM paths`
- `linux.do`
  reason: `ECH rule skipped; external real-SNI verification is only implemented for non-ECH MITM paths`
- `pincong.rocks`
  reason: `ECH rule skipped; external real-SNI verification is only implemented for non-ECH MITM paths`
- `poe.com`
  reason: `ECH rule skipped; external real-SNI verification is only implemented for non-ECH MITM paths`
- `sinyalee.com`
  reason: `ECH rule skipped; external real-SNI verification is only implemented for non-ECH MITM paths`
- `substack.com`
  reason: `ECH rule skipped; external real-SNI verification is only implemented for non-ECH MITM paths`
- `v2ex.com`
  reason: `ECH rule skipped; external real-SNI verification is only implemented for non-ECH MITM paths`
- `t.co` group=`x.com`
  reason: `ECH rule skipped; external real-SNI verification is only implemented for non-ECH MITM paths`
- `twitter.com` group=`x.com`
  reason: `ECH rule skipped; external real-SNI verification is only implemented for non-ECH MITM paths`
- `x.com`
  reason: `ECH rule skipped; external real-SNI verification is only implemented for non-ECH MITM paths`
- `z-lib.help` group=`z-lib`
  reason: `ECH rule skipped; external real-SNI verification is only implemented for non-ECH MITM paths`

