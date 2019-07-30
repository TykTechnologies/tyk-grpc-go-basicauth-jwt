# gRPC Go Plugin: Basic Auth - IdP - JWT - Microservices

A client sends a request to Tyk Gateway using basic auth:

```
curl http://foo:bar@tyk.gateway/someservice
or
curl http://tyk.gateway/someservice -H "Authorization: Basic $(echo -n foo:bar | base64)"
```

Tyk invokes the custom authentication hook and calls this gRPC plugin.

The plugin has an internal DB as a Proof-of-Concept, however you
could quite easily extend this to speak with any 3rd party IdP. E.g.
OpenLDAP / ActiveDirectory / Some other DB such as MySql or another service.

Assuming the happy path, the plugin then creates a JWT, adding the username 
to the `sub` claim. You are free to add scopes/groups/permissions or other claims
to the JWT. The gateway then signs the JWT with a HMAC Shared Secret.

Feel free to modify code to sign using RSA256 or some other algo in accordance
with your requirements.

The JWT is then stored inside the 
<a href="https://tyk.io/docs/concepts/session-meta-data/object" target="_blank">
	session object meta-data</a> for later reuse. 

We apply a 
<a href="https://tyk.io/docs/security/security-policies/policies-guide/" target="_blank">
	Tyk security policy</a> to the session object. This allows the gateway 
	to handle access rights, rate-limits & quotas.

Using Tyk's native 
<a href="https://tyk.io/docs/transform-traffic/request-headers/" target="_blank">
	Global Header Transform</a> middleware, we extract the JWT from the session
	metadata, and inject it into the Authorization header of the request.
	
The upstream service can now:

1. Validate that the request came from the gateway, by validating the signature
using the shared secret.
2. Read the claims of the JWT to determine permissions, user, grants et al.
3. Pass the JWT on to other microservices which may need it

We also configured Tyk's ID extractor to use the client's original basic-auth
credentials as a unique key. This means that Tyk Gateway no longer needs to
perform the expensive operation of calling the gRPC plugin on every single
request, only when the TTL for the id_extractor expires. Whilst not benchmarked,
this should increase performance of the plugin significantly.

As such, the same JWT will be used from the session token's metadata until
the configured expiry of the id_extractor cache, by which time, the the plugin
will be re-called by the gateway, re-authenticated with the db, and a new JWT 
generated and attached to the token meta.

---

The client is only using BasicAuth, and is oblivious to the use of JWT between
the Tyk Gateway & internal microservices.

## Quickstart:

```
go get -u github.com/TykTechnologies/tyk-grpc-go-basicauth-jwt
go install && tyk-grpc-go-basicauth-jwt

INFO[0000] starting grpc middleware on :9111 
```

Then update your gateway to point coprocess auth to grpc server

```
"coprocess_options": {
  "enable_coprocess": true,
  "coprocess_grpc_server": "tcp://:9111"
},
```

And load your API definition - example in `apidef.json` in this repo

## More detail:

gRPC server listens on:

```
listenAddress       = ":9111"
```

JWT HMAC shared secret:

```
jwtHmacSharedSecret = "foobarbaz"
```

Applied Policy - Make sure to change this for your specificy policy_id

```
policiesToApply = []string{
	"5d3f3c603f03d3d66fbfad77",
}
```

Usernames and passwords hardcoded and bootstrapped in `init()` function.
You can remove this when you replace logic for the DB.

```
func init() {
	var pass []byte
	var err error

	// bootstrapping the user DB

	pass, err = bcrypt.GenerateFromPassword([]byte("bar"), 10)
	fatalOnError(err, "unable to bootstrap db")
	userDB["foo"] = pass

	pass, err = bcrypt.GenerateFromPassword([]byte("baz"), 10)
	fatalOnError(err, "unable to bootstrap db")
	userDB["bar"] = pass
}
```

You will see in the API Definition that we utilise the ID extractor to cache & reduce load on the plugin

```
...
"response": [],
"driver": "grpc",
"id_extractor": {
  "extract_from": "header",
  "extract_with": "value",
  "extractor_config": {
    "header_name": "Authorization"
  }
}
...
```

See example api definition in `apidef.json`

The plugin stores the JWT inside the session metadata.
The Gateway in the Global Header transform pulls the JWT from the session metadata and injects it
into the header for use by the upstream.

```
"global_headers": {
  "Authorization": "Bearer $tyk_meta.jwt"
},
```
