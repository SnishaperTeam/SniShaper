package common

import "github.com/SnishaperTeam/snishaper-core/config"

// VerifyChainOnly 校验对端证书链是否可由系统根信任，不匹配主机名。
// 用于 SNI 可能被主动伪造的场景：只要求对方持有一张可信 CA 签发的证书。
// 实现已迁入 snishaper-core/config，这里保留别名。
var VerifyChainOnly = config.VerifyChainOnly
