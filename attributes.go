package main

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/labstack/echo/v5"
)

/*
	This is for both the user attributes, and the knot attributes
*/

/* Encryption: Client side using masterKey + fresh iv */
type attributesParams struct {
	Data     []byte `json:"data"`
	DataIv   []byte `json:"data_iv"`
	Revision *int64 `json:"revision"`
}

func parseAttributesBody(c *echo.Context) (attributesParams, error) {
	params := attributesParams{}
	if err := json.NewDecoder(c.Request().Body).Decode(&params); err != nil {
		return params, echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Error parsing JSON: %v", err))
	}
	if len(params.Data) == 0 || len(params.DataIv) != 24 {
		return params, echo.NewHTTPError(http.StatusBadRequest, "data required; data_iv must be 24 bytes")
	}
	return params, nil
}
