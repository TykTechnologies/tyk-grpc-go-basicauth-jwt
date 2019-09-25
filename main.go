package main

import (
	"context"
	"encoding/base64"
	"net"
	"net/http"
	"strings"
	"time"

	coprocess "github.com/TykTechnologies/tyk-protobuf/bindings/go"
	"github.com/dgrijalva/jwt-go"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc"
)

const (
	listenAddress       = ":9111"
	jwtHmacSharedSecret = "foobarbaz"
)

var (
	// user:pass
	userDB          = map[string][]byte{}
	policiesToApply = []string{
		"5d8929d8f56e1a138f628269",
	}
)

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

func main() {
	lis, err := net.Listen("tcp", listenAddress)
	fatalOnError(err, "failed to start tcp listener")

	logrus.Infof("starting grpc middleware on %s", listenAddress)
	s := grpc.NewServer()
	coprocess.RegisterDispatcherServer(s, &Dispatcher{})

	fatalOnError(s.Serve(lis), "unable to start grpc middleware")
}

type Dispatcher struct{}

func (d *Dispatcher) Dispatch(ctx context.Context, object *coprocess.Object) (*coprocess.Object, error) {
	switch object.HookName {
	case "Login":
		println("calling LoginHook")
		return LoginHook(object)
	}
	logrus.Warnf("unknown hook: %v", object.HookName)

	return object, nil
}

func (d *Dispatcher) DispatchEvent(ctx context.Context, event *coprocess.Event) (*coprocess.EventReply, error) {
	return &coprocess.EventReply{}, nil
}

func LoginHook(object *coprocess.Object) (*coprocess.Object, error) {
	authKey := object.Request.Headers["Authorization"]
	un, pw, found := parseBasicAuth(authKey)
	if !found {
		return failAuth(object, "credentials not present")
	}

	// REPLACE WITH CUSTOM LOGIC
	realPass, userExists := userDB[un]
	if !userExists {
		return failAuth(object, "user not in DB")
	}

	if err := bcrypt.CompareHashAndPassword(realPass, []byte(pw)); err != nil {
		return failAuth(object, "wrong password")
	}
	// /REPLACE WITH CUSTOM LOGIC

	jot, err := generateJWT(un)
	if err != nil {
		println("error generating jwt", err.Error())
		object.Request.ReturnOverrides.ResponseError = "middleware error"
		object.Request.ReturnOverrides.ResponseCode = http.StatusInternalServerError
		return object, nil
	}

	// Set the ID extractor deadline, useful for caching valid keys:
	extractorDeadline := time.Now().Add(time.Minute).Unix()
	object.Session = &coprocess.SessionState{
		LastUpdated:         time.Now().String(),
		Rate:                50,
		Per:                 10,
		QuotaMax:            int64(0),
		QuotaRenews:         time.Now().Unix(),
		IdExtractorDeadline: extractorDeadline,
		Metadata: map[string]string{
			"jwt":   jot,
			"token": un,
		},
		ApplyPolicies: policiesToApply,
	}

	return object, nil
}

func parseBasicAuth(auth string) (username, password string, ok bool) {
	const prefix = "Basic "
	// Case insensitive prefix match. See Issue 22736.
	if len(auth) < len(prefix) || !strings.EqualFold(auth[:len(prefix)], prefix) {
		return
	}
	c, err := base64.StdEncoding.DecodeString(auth[len(prefix):])
	if err != nil {
		return
	}
	cs := string(c)
	s := strings.IndexByte(cs, ':')
	if s < 0 {
		return
	}
	return cs[:s], cs[s+1:], true
}

func generateJWT(username string) (string, error) {
	token := jwt.New(jwt.SigningMethodHS256)

	token.Claims = &jwt.StandardClaims{
		Subject:  username,
		IssuedAt: time.Now().Unix(),
	}

	return token.SignedString([]byte(jwtHmacSharedSecret))
}

func fatalOnError(err error, msg string) {
	if err != nil {
		logrus.WithError(err).Fatal(msg)
	}
}

func failAuth(object *coprocess.Object, msg string) (*coprocess.Object, error) {
	object.Request.ReturnOverrides.ResponseCode = http.StatusForbidden
	object.Request.ReturnOverrides.ResponseError = msg
	return object, nil
}
