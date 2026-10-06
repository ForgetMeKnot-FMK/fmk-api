package main

import (
	"crypto/sha256"
	"errors"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/ishraq10199/fmk-api/internal/database"
	"github.com/labstack/echo/v5"
)

// getIP returns the ip address from the http request
func getIP(r *http.Request) (string, error) {
	ips := r.Header.Get("X-Forwarded-For")
	splitIps := strings.Split(ips, ",")

	if len(splitIps) > 0 {
		// get last IP in list since ELB prepends other user defined IPs, meaning the last one is the actual client IP.
		netIP := net.ParseIP(splitIps[len(splitIps)-1])
		if netIP != nil {
			return netIP.String(), nil
		}
	}

	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return "", err
	}

	netIP := net.ParseIP(ip)
	if netIP != nil {
		ip := netIP.String()
		if ip == "::1" {
			return "127.0.0.1", nil
		}
		return ip, nil
	}

	return "", errors.New("IP not found")
}

func getSessionToken(headers *http.Header) ([]byte, error) {
	val := headers.Get("Authorization")
	if val == "" {
		return []byte{}, errors.New("no credentials found")
	}
	vals := strings.Split(val, " ")
	if len(vals) != 2 || vals[0] != "Bearer" {
		return []byte{}, errors.New("malformed auth header")
	}
	return []byte(vals[1]), nil
}

// How often a session's last_used_at is written: not on every request
const sessionTouchInterval = time.Minute

/*
getAuthFromRequest finds the user and the session of a request's Bearer token.

Steps:
 1. Read the token from the Authorization header.
 2. Look up its SHA-256 among live sessions (not revoked, not expired).
 3. If the session was last used more than a minute ago, note that it is used
    now. A failure here doesn't fail the request.

Example: a request with a revoked session's token gets pgx.ErrNoRows.
*/
func (apiCfg *apiConfig) getAuthFromRequest(e *echo.Context) (database.GetAuthByTokenHashRow, error) {
	token, err := getSessionToken(&e.Request().Header)
	if err != nil {
		return database.GetAuthByTokenHashRow{}, err
	}
	ctx := e.Request().Context()
	tokenHash := sha256.Sum256(token)
	auth, err := apiCfg.DB.GetAuthByTokenHash(ctx, tokenHash[:])
	if err != nil {
		return auth, err
	}
	if time.Since(auth.SessionLastUsedAt.Time) > sessionTouchInterval {
		if err := apiCfg.DB.TouchSession(ctx, auth.SessionID); err != nil {
			log.Println("warn: could not update last_used_at:", err)
		}
	}
	return auth, nil
}

func (apiCfg *apiConfig) getUserFromRequest(e *echo.Context) (database.User, error) {
	auth, err := apiCfg.getAuthFromRequest(e)
	return auth.User, err
}
