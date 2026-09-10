package monpay

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"time"
)

// ExternalToken is a merchant token whose lifetime the CALLER owns.
//
// It is the opt-in alternative to this SDK's built-in token management. By
// default the client still mints, caches and refreshes the client-credentials
// token itself (see [Deeplink.Auth] and the managed path in
// httpRequestDeeplink); nothing about that changes unless an ExternalToken is
// installed with [Deeplink.UseExternalToken] or [WithExternalToken].
//
// Install one when the token lives outside this process — in a shared cache,
// or minted by another service — and the SDK must not mint a second one behind
// your back. While one is installed the client sends it verbatim and never
// re-mints or refreshes it: a token it does not own is not its to replace, and
// doing so would be invisible to whoever does own it. A rejected token surfaces
// as [ErrUnauthorized] for the caller to act on.
type ExternalToken struct {
	// AccessToken is the bearer sent as Authorization on merchant calls.
	AccessToken string

	// RefreshToken is returned by Monpay. There is no refresh endpoint on the
	// merchant flow, so it is carried for completeness only: renew with
	// [Deeplink.FetchToken].
	RefreshToken string

	// TokenType is Monpay's token_type, normally "Bearer".
	TokenType string

	// ExpiresAt is when AccessToken stops being accepted.
	//
	// Monpay's client-credentials token often carries no expires_in at all, in
	// which case this is the ZERO time: the gateway told us nothing, so the SDK
	// guesses nothing. A caller holding a zero ExpiresAt should apply its own
	// renewal interval — the managed path uses defaultTokenTTL for exactly this
	// case, and is a reasonable thing to mirror.
	ExpiresAt time.Time

	// Scope is returned verbatim for diagnostics.
	Scope string
}

// IsZero reports whether the token carries no credential at all.
func (t ExternalToken) IsZero() bool { return strings.TrimSpace(t.AccessToken) == "" }

// ErrUnauthorized is returned when Monpay rejects an installed [ExternalToken]
// with 401 or 403. The caller should discard the token, obtain a new one and
// retry — the rejected request was never processed, so retrying cannot
// double-create an invoice.
//
// The managed path does not return this: there the SDK owns the token and
// refreshes it itself.
var ErrUnauthorized = errors.New("monpay: external access token rejected")

// FetchToken mints a merchant token and returns it WITHOUT installing or
// caching it — exactly one request, whatever Monpay said.
//
// It is the explicit counterpart to the managed path's getAccessToken: no
// cache lookup, no singleflight deduplication, no background renewal.
// Concurrent callers each issue their own request, so collapsing them is the
// caller's job, as is deciding when to replace the result.
func (d *deeplink) FetchToken(ctx context.Context) (ExternalToken, error) {
	// The app/merchant token is always a client-credentials grant; grantType
	// belongs to the user authorization_code flow in Auth. Same rule as the
	// managed path.
	formBody := url.Values{}
	formBody.Add("client_id", d.clientId)
	formBody.Add("client_secret", d.clientSecret)
	formBody.Add("grant_type", clientCredentialsGrant(d.grantType))

	authToken, err := d.doTokenRequestCtx(ctx, formBody)
	if err != nil {
		return ExternalToken{}, err
	}
	return externalTokenFrom(authToken, time.Now()), nil
}

// UseExternalToken installs the token every subsequent merchant call carries,
// and switches the client out of managed mode.
//
// Passing the zero ExternalToken clears it and hands token management back to
// the SDK — the client resumes minting and refreshing its own.
func (d *deeplink) UseExternalToken(token ExternalToken) {
	d.mu.Lock()
	d.externalToken = token
	d.mu.Unlock()
}

// InstalledToken returns the token installed by [Deeplink.UseExternalToken].
// It is the zero ExternalToken when the client is in managed mode.
func (d *deeplink) InstalledToken() ExternalToken {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.externalToken
}

// externalTokenFrom converts a client-credentials response into an
// ExternalToken.
//
// Monpay sends expires_in as a duration in seconds, so the expiry is anchored
// to when we received it — which is why this conversion happens at the point
// of the response and not later. No expires_in yields the zero time; see
// [ExternalToken.ExpiresAt].
func externalTokenFrom(res AccessToken, now time.Time) ExternalToken {
	var expiresAt time.Time
	if res.ExpiresIn > 0 {
		expiresAt = now.Add(time.Duration(res.ExpiresIn) * time.Second)
	}

	return ExternalToken{
		AccessToken:  res.AccessToken,
		RefreshToken: res.RefreshToken,
		TokenType:    res.TokenType,
		ExpiresAt:    expiresAt,
		Scope:        res.Scope,
	}
}
