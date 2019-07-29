# Custom Auth Middleware Basic Auth to JWT Upstream Auth

gRPC Plugin which accepts BasicAuth from client, validates the credentials, then if successful, generates a JWT signed with shared secret for use by the upstream.

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
},
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
