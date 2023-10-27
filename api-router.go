/*
 * MinIO Cloud Storage, (C) 2016-2020 MinIO, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package miniocmd

import (
	"net"
	"net/http"

	"github.com/gorilla/mux"
	xhttp "github.com/minio/minio/cmd/http"
	"github.com/minio/minio/pkg/wildcard"
	"github.com/rs/cors"
)

func newHTTPServerFn() *xhttp.Server {
	globalObjLayerMutex.RLock()
	defer globalObjLayerMutex.RUnlock()
	return globalHTTPServer
}

func setHTTPServer(h *xhttp.Server) {
	globalObjLayerMutex.Lock()
	globalHTTPServer = h
	globalObjLayerMutex.Unlock()
}

func newObjectLayerFn() ObjectLayer {
	globalObjLayerMutex.RLock()
	defer globalObjLayerMutex.RUnlock()
	return globalObjectAPI
}

func newCachedObjectLayerFn() CacheObjectLayer {
	globalObjLayerMutex.RLock()
	defer globalObjLayerMutex.RUnlock()
	return globalCacheObjectAPI
}

func setCacheObjectLayer(c CacheObjectLayer) {
	globalObjLayerMutex.Lock()
	globalCacheObjectAPI = c
	globalObjLayerMutex.Unlock()
}

func setObjectLayer(o ObjectLayer) {
	globalObjLayerMutex.Lock()
	globalObjectAPI = o
	globalObjLayerMutex.Unlock()
}

// objectAPIHandler implements and provides http handlers for S3 API.
type objectAPIHandlers struct {
	ObjectAPI func() ObjectLayer
	CacheAPI  func() CacheObjectLayer
}

// getHost tries its best to return the request host.
// According to section 14.23 of RFC 2616 the Host header
// can include the port number if the default value of 80 is not used.
func getHost(r *http.Request) string {
	if r.URL.IsAbs() {
		return r.URL.Host
	}
	return r.Host
}

func notImplementedHandler(w http.ResponseWriter, r *http.Request) {
	writeErrorResponse(r.Context(), w, errorCodes.ToAPIErr(ErrNotImplemented), r.URL, guessIsBrowserReq(r))
}

type rejectedAPI struct {
	api     string
	methods []string
	queries []string
	path    string
}

var rejectedAPIs = []rejectedAPI{
	{
		api:     "inventory",
		methods: []string{http.MethodGet, http.MethodPut, http.MethodDelete},
		queries: []string{"inventory", ""},
	},
	{
		api:     "cors",
		methods: []string{http.MethodPut, http.MethodDelete},
		queries: []string{"cors", ""},
	},
	{
		api:     "metrics",
		methods: []string{http.MethodGet, http.MethodPut, http.MethodDelete},
		queries: []string{"metrics", ""},
	},
	{
		api:     "website",
		methods: []string{http.MethodPut},
		queries: []string{"website", ""},
	},
	{
		api:     "logging",
		methods: []string{http.MethodPut, http.MethodDelete},
		queries: []string{"logging", ""},
	},
	{
		api:     "accelerate",
		methods: []string{http.MethodPut, http.MethodDelete},
		queries: []string{"accelerate", ""},
	},
	{
		api:     "requestPayment",
		methods: []string{http.MethodPut, http.MethodDelete},
		queries: []string{"requestPayment", ""},
	},
	{
		api:     "torrent",
		methods: []string{http.MethodPut, http.MethodDelete, http.MethodGet},
		queries: []string{"torrent", ""},
		path:    "/{object:.+}",
	},
	{
		api:     "acl",
		methods: []string{http.MethodDelete},
		queries: []string{"acl", ""},
		path:    "/{object:.+}",
	},
	{
		api:     "acl",
		methods: []string{http.MethodDelete, http.MethodPut, http.MethodHead},
		queries: []string{"acl", ""},
	},
	{
		api:     "publicAccessBlock",
		methods: []string{http.MethodDelete, http.MethodPut, http.MethodGet},
		queries: []string{"publicAccessBlock", ""},
	},
	{
		api:     "ownershipControls",
		methods: []string{http.MethodDelete, http.MethodPut, http.MethodGet},
		queries: []string{"ownershipControls", ""},
	},
	{
		api:     "intelligent-tiering",
		methods: []string{http.MethodDelete, http.MethodPut, http.MethodGet},
		queries: []string{"intelligent-tiering", ""},
	},
	{
		api:     "analytics",
		methods: []string{http.MethodDelete, http.MethodPut, http.MethodGet},
		queries: []string{"analytics", ""},
	},
}

func rejectUnsupportedAPIs(router *mux.Router) {
	for _, r := range rejectedAPIs {
		t := router.Methods(r.methods...).
			HandlerFunc(collectAPIStats(r.api, httpTraceAll(notImplementedHandler))).
			Queries(r.queries...)
		if r.path != "" {
			t.Path(r.path)
		}
	}
}

// registerAPIRouter - registers S3 compatible APIs.
func registerAPIRouter(router *mux.Router) {

	// API Router
	apiRouter := router.PathPrefix(SlashSeparator).Subrouter()

	var routers []*mux.Router
	for _, domainName := range globalDomainNames {
		if IsKubernetes() {
			routers = append(routers, apiRouter.MatcherFunc(func(r *http.Request, match *mux.RouteMatch) bool {
				host, _, err := net.SplitHostPort(getHost(r))
				if err != nil {
					host = r.Host
				}
				// Make sure to skip matching minio.<domain>` this is
				// specifically meant for operator/k8s deployment
				// The reason we need to skip this is for a special
				// usecase where we need to make sure that
				// minio.<namespace>.svc.<cluster_domain> is ignored
				// by the bucketDNS style to ensure that path style
				// is available and honored at this domain.
				//
				// All other `<bucket>.<namespace>.svc.<cluster_domain>`
				// makes sure that buckets are routed through this matcher
				// to match for `<bucket>`
				return host != minioReservedBucket+"."+domainName
			}).Host("{bucket:.+}."+domainName).Subrouter())
		} else {
			routers = append(routers, apiRouter.Host("{bucket:.+}."+domainName).Subrouter())
		}
	}
	routers = append(routers, apiRouter.PathPrefix("/{bucket}").Subrouter())

	for _, router := range routers {
		rejectUnsupportedAPIs(router)
	}

	// If none of the routes match add default error handler routes
	apiRouter.NotFoundHandler = collectAPIStats("notfound", httpTraceAll(errorResponseHandler))
	apiRouter.MethodNotAllowedHandler = collectAPIStats("methodnotallowed", httpTraceAll(methodNotAllowedHandler("S3")))

}

// corsHandler handler for CORS (Cross Origin Resource Sharing)
func corsHandler(handler http.Handler) http.Handler {
	commonS3Headers := []string{
		xhttp.Date,
		xhttp.ETag,
		xhttp.ServerInfo,
		xhttp.Connection,
		xhttp.AcceptRanges,
		xhttp.ContentRange,
		xhttp.ContentEncoding,
		xhttp.ContentLength,
		xhttp.ContentType,
		xhttp.ContentDisposition,
		xhttp.LastModified,
		xhttp.ContentLanguage,
		xhttp.CacheControl,
		xhttp.RetryAfter,
		xhttp.AmzBucketRegion,
		xhttp.Expires,
		"X-Amz*",
		"x-amz*",
		"*",
	}

	return cors.New(cors.Options{
		AllowOriginFunc: func(origin string) bool {
			for _, allowedOrigin := range globalAPIConfig.getCorsAllowOrigins() {
				if wildcard.MatchSimple(allowedOrigin, origin) {
					return true
				}
			}
			return false
		},
		AllowedMethods: []string{
			http.MethodGet,
			http.MethodPut,
			http.MethodHead,
			http.MethodPost,
			http.MethodDelete,
			http.MethodOptions,
			http.MethodPatch,
		},
		AllowedHeaders:   commonS3Headers,
		ExposedHeaders:   commonS3Headers,
		AllowCredentials: true,
	}).Handler(handler)
}
