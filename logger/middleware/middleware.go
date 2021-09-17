package middleware

import (
	"github.com/labstack/echo/v4"
	"github.com/tailed0/isucon-library/logger"
)

var alptrace = logger.New()

func Middleware() func(next echo.HandlerFunc) echo.HandlerFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			t := alptrace.Start()
			err := next(c)
			t.Stop(c.Request().Method, c.Path(), c.Response().Status, c.Response().Size)
			return err
		}
	}

}
