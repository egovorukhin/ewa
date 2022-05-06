package egowebapi

import (
	"github.com/egovorukhin/egowebapi/consts"
	"github.com/egovorukhin/egowebapi/security"
	"net/http"
	"strconv"
	"time"
)

type Route struct {
	isEmptyParam bool
	session      SessionTurn
	isPermission bool
	Handler      Handler
	Operation
}

// Map тип список
type Map map[string]interface{}

type SessionTurn int

const (
	None SessionTurn = iota
	Is
	On
	Off
)

// SetParameters указываем параметры маршрута
func (r *Route) SetParameters(isEmptyParam bool, params ...*Parameter) *Route {
	r.isEmptyParam = isEmptyParam
	for _, param := range params {
		if param.In == InPath {
			param.AllowEmptyValue = isEmptyParam
		}
		r.Parameters = append(r.Parameters, param)
	}
	return r
}

// SetConsumes устанавливаем Content-Type запроса для Swagger
func (r *Route) SetConsumes(c ...string) *Route {
	r.Consumes = c
	return r
}

// SetProduces устанавливаем Content-Type ответа для Swagger
func (r *Route) SetProduces(p ...string) *Route {
	r.Produces = p
	return r
}

// SetOperationID устанавливаем идентификатор операции для Swagger
func (r *Route) SetOperationID(id string) *Route {
	r.ID = id
	return r
}

// SetDefaultResponse описываем варианты ответов для Swagger
func (r *Route) SetDefaultResponse(resp Response) *Route {
	r.Responses["default"] = resp
	return r
}

// SetResponse описываем варианты ответов для Swagger
func (r *Route) SetResponse(code int, resp Response) *Route {
	r.Responses[strconv.Itoa(code)] = resp
	return r
}

// SetDescription описание операции
func (r *Route) SetDescription(desc string) *Route {
	r.Description = desc
	return r
}

// SetSummary резюме запроса
func (r *Route) SetSummary(s string) *Route {
	r.Summary = s
	return r
}

// SetSign устанавливаем вариант входа/выхода для маршрута
/*func (r *Route) SetSign(sign Sign) *Route {
	r.sign = sign
	return r
}*/

// SetSecurity указываем метод авторизации
func (r *Route) SetSecurity(security ...string) *Route {
	for _, sec := range security {
		r.Security = append(r.Security, map[string][]string{
			sec: {},
		})
	}
	//r.auth = auth
	return r
}

// Session вешаем получение аутентификации сессии,
func (r *Route) Session(t ...SessionTurn) *Route {
	if t == nil {
		r.session = Is
	} else {
		r.session = t[0]
	}
	return r
}

// Permission ставим флаг для проверки маршрута на право доступа
func (r *Route) Permission() *Route {
	r.isPermission = true
	return r
}

// EmptyHandler пустой обработчик
func (r *Route) EmptyHandler() {
	r.Handler = nil
}

// SetHandler устанавливаем обработчик
func (r *Route) SetHandler(handler Handler) *Route {
	r.Handler = handler
	return r
}

// getHandler возвращаем обработчик основанный на параметрах конфигурации маршрута
func (r *Route) getHandler(config Config, view *View, swagger Swagger) Handler {

	return func(c *Context) error {

		c.View = view
		c.Swagger = swagger

		var (
			err        error
			isSecurity bool
		)
		for _, sec := range r.Security {
			for key := range sec {
				switch key {
				case security.BasicAuth:
					if config.Authorization.Basic != nil {
						config.Authorization.Basic.SetHeader(c.Get(consts.HeaderAuthorization))
						c.Identity, err = config.Authorization.Basic.Do()
						if err != nil {
							c.Set(consts.HeaderWWWAuthenticate, err.Error())
						}
					}
				case security.DigestAuth:
					if config.Authorization.Digest != nil {
						c.Identity, err = config.Authorization.Digest.Do()
					}
				case security.ApiKeyAuth:
					if config.Authorization.ApiKey != nil {
						a := config.Authorization.ApiKey
						var value string
						switch a.Param {
						// Если не нашли в заголовке, то ищем в переменных запроса адресной строки
						case security.ParamQuery:
							value = c.QueryParam(a.KeyName)
							break
						// Пытаемся получить из заголовка токен
						case security.ParamHeader:
							value = c.Get(a.KeyName)
							break
						}
						c.Identity, err = a.SetValue(value).Do()
					}
				}
				if err == nil {
					isSecurity = true
				}
			}
		}

		// Проверка на сессию
		if config.Session != nil && r.session != None {
			keyName := config.Session.KeyName
			switch r.session {
			case Is:
				if isSecurity {
					break
				}
				c.Identity, err = config.Session.Check(c.Cookies(keyName))
				if c.Session != nil {
					c.Session.LastTime = time.Now()
				}
			case On:
				value := config.Session.GenSessionIdHandler()
				cookie := &http.Cookie{
					Name:    keyName,
					Value:   value,
					Expires: time.Now().Add(config.Session.Expires),
				}
				c.SetCookie(cookie)
				now := time.Now()
				c.Session = &Session{
					Key:      keyName,
					Value:    value,
					Created:  now,
					LastTime: now,
				}
			case Off:
				c.Identity, err = config.Session.Check(c.Cookies(keyName))
				c.ClearCookie(config.Session.KeyName)
				c.Session = nil
				return c.Redirect(config.Session.RedirectPath, config.Session.RedirectStatus)
			}
		}

		// Проверка на ошибку авторизации и отправку кода 401
		if err != nil {
			if isSecurity {
				if config.Authorization.Unauthorized != nil && config.Authorization.Unauthorized(err) {
					return c.SendString(consts.StatusUnauthorized, err.Error())
				}
				return c.SendStatus(consts.StatusUnauthorized)
			} else if r.session != None {
				// Если cookie не существует, то перенаправляем запрос условно на "/login"
				return c.Redirect(config.Session.RedirectPath, config.Session.RedirectStatus)
			}
		}

		// Доступ к маршрутам
		if r.isPermission && config.Permission != nil {
			if c.Identity != nil {
				if !config.Permission.check(c.Identity.Username, c.Path()) {
					if config.Permission.NotPermissionHandler != nil {
						return config.Permission.NotPermissionHandler(c, consts.StatusForbidden, "Forbidden")
					}
					return c.SendStatus(consts.StatusForbidden)
				}
			}
		}

		// Обычный маршрут
		return r.Handler(c)
	}
}
