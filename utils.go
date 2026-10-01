package main

import (
	"context"
	"crypto/sha256"
	"errors"
	"net"
	"net/http"
	"strings"

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

func (apiCfg *apiConfig) getUserBySessionToken(ctx context.Context, st []byte) (database.User, error) {
	// token := base64.RawURLEncoding.EncodeToString(st)
	tokenHash := sha256.Sum256([]byte(st))
	return apiCfg.DB.GetUserByTokenHash(ctx, tokenHash[:])
}

func (apiConfig *apiConfig) getUserFromRequest(e *echo.Context) (database.User, error) {
	token, err := getSessionToken(&e.Request().Header)
	var user database.User
	if err != nil {
		return user, err
	}
	return apiConfig.getUserBySessionToken(e.Request().Context(), token)
}
