package api

import (
	ewa "github.com/egovorukhin/egowebapi"
	"github.com/gofiber/fiber/v2"
)

type Messages struct {
}

func (u *Messages) Get(route *ewa.Route) {
	route.Empty()
}

func (u *Messages) Post(route *ewa.Route) {
	route.Empty()
}

func (u *Messages) Put(route *ewa.Route) {
	route.Empty()
}

func (u *Messages) Delete(route *ewa.Route) {
	route.Empty()
}

func (u *Messages) Options(swagger *ewa.Swagger) ewa.EmptyHandler {
	return func(ctx *fiber.Ctx) error {
		//ctx.Append("Allow", "GET, POST, PUT, DELETE, OPTIONS")
		swagger.Allow(ctx)
		return ctx.JSON(swagger)
	}
}
