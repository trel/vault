package http

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"strings"

	"github.com/hashicorp/vault/sdk/logical"

	"github.com/hashicorp/vault/helper/namespace"
	"github.com/hashicorp/vault/vault"
	"github.com/hashicorp/vault/vault/quotas"
)

var (
	adjustRequest = func(c *vault.Core, r *http.Request) (*http.Request, int) {
		return r, 0
	}

	genericWrapping = func(core *vault.Core, in http.Handler, props *vault.HandlerProperties) http.Handler {
		// Wrap the help wrapped handler with another layer with a generic
		// handler
		return wrapGenericHandler(core, in, props)
	}

	additionalRoutes = func(mux *http.ServeMux, core *vault.Core) {}

	nonVotersAllowed = false

	adjustResponse = func(core *vault.Core, w http.ResponseWriter, req *logical.Request) {}
)

func rateLimitQuotaWrapping(handler http.Handler, core *vault.Core) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ns, err := namespace.FromContext(r.Context())
		if err != nil {
			respondError(w, http.StatusInternalServerError, err)
			return
		}

		// We don't want to do buildLogicalRequestNoAuth here because, if the
		// request gets allowed by the quota, the same function will get called
		// again, which is not desired.
		path, status, err := buildLogicalPath(r)
		if err != nil || status != 0 {
			respondError(w, status, err)
			return
		}
		mountPath := strings.TrimPrefix(core.MatchingMount(r.Context(), path), ns.Path)

		// Clone body, so we do not close the request body reader
		bodyBytes, err := ioutil.ReadAll(r.Body)
		if err != nil {
			respondError(w, http.StatusInternalServerError, errors.New("failed to read request body"))
			return
		}
		r.Body = ioutil.NopCloser(bytes.NewBuffer(bodyBytes))

		quotaResp, err := core.ApplyRateLimitQuota(r.Context(), &quotas.Request{
			Type:          quotas.TypeRateLimit,
			Path:          path,
			MountPath:     mountPath,
			Role:          core.DetermineRoleFromLoginRequestFromBytes(mountPath, bodyBytes, r.Context()),
			NamespacePath: ns.Path,
			ClientAddress: parseRemoteIPAddress(r),
		})
		if err != nil {
			core.Logger().Error("failed to apply quota", "path", path, "error", err)
			respondError(w, http.StatusUnprocessableEntity, err)
			return
		}

		if core.RateLimitResponseHeadersEnabled() {
			for h, v := range quotaResp.Headers {
				w.Header().Set(h, v)
			}
		}

		if !quotaResp.Allowed {
			quotaErr := fmt.Errorf("request path %q: %w", path, quotas.ErrRateLimitQuotaExceeded)
			respondError(w, http.StatusTooManyRequests, quotaErr)

			if core.Logger().IsTrace() {
				core.Logger().Trace("request rejected due to rate limit quota violation", "request_path", path)
			}

			if core.RateLimitAuditLoggingEnabled() {
				req, _, status, err := buildLogicalRequestNoAuth(core.PerfStandby(), w, r)
				if err != nil || status != 0 {
					respondError(w, status, err)
					return
				}

				err = core.AuditLogger().AuditRequest(r.Context(), &logical.LogInput{
					Request:  req,
					OuterErr: quotaErr,
				})
				if err != nil {
					core.Logger().Warn("failed to audit log request rejection caused by rate limit quota violation", "error", err)
				}
			}

			return
		}

		handler.ServeHTTP(w, r)
		return
	})
}

func parseRemoteIPAddress(r *http.Request) string {
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return ""
	}

	return ip
}

// Workaround an incompatibility between Go's HTTP mux and a standard OCSP GET request that
// encodes in standard base64 and places that in the URL. Go's mux will try to canonicalize
// path components by changing repeated '/' into a single '/', which corrupts the base64
// encoding of an OCSP request.
func ocspGetWrappedHandler(handler http.Handler, core *vault.Core) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet &&
			strings.Contains(r.URL.Path, "//") &&
			strings.Contains(r.URL.Path, "ocsp/") {

			ns, err := namespace.FromContext(r.Context())
			if err != nil {
				respondError(w, http.StatusInternalServerError, err)
				return
			}

			// we ignore errors as the higher layers will deal with it
			logicalPath, _, _ := buildLogicalPath(r)
			if logicalPath != "" {
				nsPath := ns.Path + logicalPath
				mountType, mountPath, found := core.GetMountTypeByAPIPath(r.Context(), nsPath)

				if found && mountType == "pki" {
					fullMountPath := ns.Path + mountPath
					base64Request := strings.TrimPrefix(nsPath, fullMountPath+"ocsp/")
					base64Request = strings.TrimPrefix(base64Request, fullMountPath+"unified-ocsp/")

					if base64Request != nsPath {
						// So one of our special OCSP paths matched the logical path and we
						// contain a character sequence that will cause us to redirect, stash the original
						// request into a GET Query URL instead and rewrite the path so the standard
						// mux does not redirect us.
						r.URL.RawQuery = fmt.Sprintf("ocspReq=%s", base64Request)
						r.URL.Path = strings.TrimSuffix(r.URL.Path, base64Request)
					}
				}
			}
		}

		handler.ServeHTTP(w, r)
		return
	})
}
