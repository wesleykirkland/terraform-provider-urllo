// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package client

import "net/url"

// nextPageQuery extracts the query parameters from a pagination "next" link so
// the caller can re-request the same collection path with the cursor advanced.
// It returns ok=false when there is no further page. Only the query is reused
// (not the path), which keeps pagination independent of the API version prefix
// baked into the link.
func nextPageQuery(next *string) (url.Values, bool) {
	if next == nil || *next == "" {
		return nil, false
	}
	u, err := url.Parse(*next)
	if err != nil {
		return nil, false
	}
	q := u.Query()
	if len(q) == 0 {
		return nil, false
	}
	return q, true
}
