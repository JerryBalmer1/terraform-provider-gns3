// Package gns3 is a hand-written REST client for the GNS3 controller API.
//
// NewV2 returns a Client for GNS3 server 2.2.x: it speaks the REST /v2/...
// endpoints and authenticates with HTTP Basic auth when Config.Username is
// set. Client is version-neutral; a GNS3 3.x implementation (/v3/..., JWT
// bearer token) is meant to be a second type satisfying the same interface,
// not a change to this one. Every non-2xx response is returned as *APIError.
//
// The package is the client the terraform-provider-gns3 provider uses, and it
// is stable for use by other Go modules. It is versioned with the provider and
// follows semantic versioning: breaking changes to its exported API come only
// with a new major version.
//
// Usage:
//
//	c := gns3.NewV2(gns3.Config{Host: "127.0.0.1", Port: 3080})
//	v, err := c.GetVersion(context.Background())
//	if err != nil {
//		log.Fatal(err)
//	}
//	fmt.Println(v.Version)
//
// Endpoint reference: GNS3 server 2.2.52 controller API,
// https://gns3-server.readthedocs.io/en/stable/endpoints.html
// (docs.gns3.com/apiv2 returned 404 when this client was written; the
// readthedocs pages are the same API reference). Each method cites its page.
package gns3
