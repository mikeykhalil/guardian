package guardian

import "net/url"

type GlobalLimitProvider struct {
	*RedisConfStore
}

type RouteLimitProvider struct {
	*RedisConfStore
}

func NewGlobalLimitProvider(rcs *RedisConfStore) *GlobalLimitProvider {
	return &GlobalLimitProvider{rcs}
}

func (glp *GlobalLimitProvider) GetLimit(_ Request) Limit {
	glp.conf.RLock()
	defer glp.conf.RUnlock()

	return glp.conf.limit
}

func NewRouteRateLimitProvider(rcs *RedisConfStore) *RouteLimitProvider {
	return &RouteLimitProvider{rcs}
}

func (rlp *RouteLimitProvider) GetLimit(req Request) (limit Limit) {
	reqUrl, err := url.Parse(req.Path)
	if err != nil || reqUrl == nil {
		rlp.logger.Warnf("unable to parse url from request: %v", err)
		return
	}

	rlp.conf.RLock()
	defer rlp.conf.RUnlock()
	limit = rlp.conf.routeRateLimits[*reqUrl]
	return
}
