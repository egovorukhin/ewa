package egowebapi

import (
	"fmt"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/session"
	"github.com/valyala/fasthttp"
	"os"
	p "path"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"time"
)

type Server struct {
	*fiber.App
	Name      string
	IsStarted bool
	Config    Config
	Swagger   *Swagger
}

type Suffix struct {
	Index int
	Value string
}

func NewSuffix(suffix ...Suffix) (s []Suffix) {
	for _, item := range suffix {
		s = append(s, item)
	}
	return
}

type Cors cors.Config
type Store session.Config

type IServer interface {
	Start() error
	Stop() error
	Register(i interface{}, path string) *Server
	RegisterExt(i interface{}, path string, name string, suffix ...Suffix) *Server
	SetCors(config *Cors) *Server
	GetApp() *fiber.App
	//SetStore(config *Store) * Server
}

func New(name string, config Config) (IServer, error) {

	//Таймауты
	readTimeout, writeTimeout, idleTimeout := config.Timeout.Get()
	// Буферы
	readBufferSize, writeBufferSize := config.BufferSize.Get()
	//Получаем расположение исполняемого файла
	exePath, err := os.Executable()
	if err != nil {
		return nil, err
	}
	//Настройки
	settings := fiber.Config{
		BodyLimit:       config.BodyLimit,
		ReadTimeout:     time.Duration(readTimeout) * time.Second,
		WriteTimeout:    time.Duration(writeTimeout) * time.Second,
		IdleTimeout:     time.Duration(idleTimeout) * time.Second,
		ReadBufferSize:  readBufferSize,
		WriteBufferSize: writeBufferSize,
	}
	//Указываем нужны ли страницы
	if config.Views != nil {
		if config.Views.Extension != None {
			settings.Views = config.Views.Extension.Engine(filepath.Join(filepath.Dir(exePath), config.Views.Directory), config.Views.Engine)
		}
		if config.Views.Layout != "" {
			settings.ViewsLayout = config.Views.Layout
		}
	}
	//Инициализируем сервер
	server := fiber.New(settings)
	//Устанавливаем статические файлы
	if config.Static != nil {
		prefix := "/"
		if config.Static.Prefix != "" {
			prefix = config.Static.Prefix
		}
		server.Static(prefix, filepath.Join(filepath.Dir(exePath), config.Static.Root))
	}

	return &Server{
		Name:   name,
		Config: config,
		App:    server,
	}, nil
}

func (s *Server) GetApp() *fiber.App {
	return s.App
}

func (s *Server) Start() error {

	//Флаг старта
	s.IsStarted = true

	addr := fmt.Sprintf(":%d", s.Config.Port)

	//Если Secure == nil, то запускаем без сертификата
	if s.Config.Secure != nil {
		// Возвращаем данные по сертификату
		cert, key := s.Config.Secure.Get()
		// Запускаем слушатель с TLS настройкой
		if err := s.ListenTLS(addr, cert, key); err != fasthttp.ErrConnectionClosed {
			return err
		}
	} else {
		//Запускаем слушатель
		if err := s.Listen(addr); err != fasthttp.ErrConnectionClosed {
			return err
		}
	}

	return nil
}

// Устанавливаем глобальные настройки для маршрутов
func (s *Server) newRoute() *Route {
	route := new(Route)
	if s.Config.Session != nil {
		route.IsSession = s.Config.Session.AllRoutes
	}
	if s.Config.Permission != nil {
		route.IsPermission = s.Config.Permission.AllRoutes
	}
	if s.Config.Authorization.AllRoutes != "" {
		route.Authorization = s.Config.Authorization.AllRoutes
	} else {
		route.Authorization = NoAuth
	}
	return route
}

// Обрабатываем метод GET
func (s *Server) get(i IGet, name, path string) {
	route := s.newRoute()
	i.Get(route)
	s.add(fiber.MethodGet, name, path, route)
}

// Обрабатываем метод POST
func (s *Server) post(i IPost, name, path string) {
	route := s.newRoute()
	i.Post(route)
	s.add(fiber.MethodPost, name, path, route)
}

// Обрабатываем метод PUT
func (s *Server) put(i IPut, name, path string) {
	route := s.newRoute()
	i.Put(route)
	s.add(fiber.MethodPut, name, path, route)
}

// Обрабатываем метод DELETE
func (s *Server) delete(i IDelete, name, path string) {
	route := s.newRoute()
	i.Delete(route)
	s.add(fiber.MethodDelete, name, path, route)
}

// Обрабатываем метод OPTIONS
func (s *Server) options(i IRestOptions, name, path string) {
	route := s.newRoute()
	i.Options(route)
	s.add(fiber.MethodOptions, name, path, route)
}

// Обрабатываем интерфейс IWeb
func (s *Server) web(i IWeb, name, path string) {
	s.get(i, name, path)
	s.post(i, name, path)
}

// Обрабатываем интерфейс IRest
func (s *Server) rest(i IRest, name, path string) {
	s.web(i, name, path)
	s.put(i, name, path)
	s.delete(i, name, path)
}

// Обрабатываем интерфейс IRestOptions
func (s *Server) restOptions(i IRestOptions, name, path string) {
	s.rest(i, name, path)
	s.options(i, name, path)
}

func (s *Server) add(method string, name, path string, route *Route) {

	// Если нет ни одного handler, то выходим
	if route == nil || route.Handler == nil || method == "" {
		return
	}

	if route.Params == nil {
		route.Params = []string{"", "/"}
	} else {
		// Проверка пути на пустоту и слэш
		emptyPath := false
		slash := false
		for _, param := range route.Params {
			switch param {
			case "":
				emptyPath = true
				break
			case "/":
				slash = true
				break
			}
		}
		if emptyPath && !slash {
			route.Params = append(route.Params, "/")
		} else if !emptyPath && slash {
			route.Params = append(route.Params, "")
		}
	}
	route.Option.Method = method

	// Инициализируем Swagger
	if s.Swagger == nil {
		http := "http"
		if s.Config.Secure != nil {
			http = "https"
		}
		addr := "127.0.0.1"
		s.Swagger = &Swagger{
			Uri: fmt.Sprintf("%s://%s:%d", http, addr, s.Config.Port),
		}
	}

	// WebSocket
	if route.webSocket != nil && route.webSocket.UpgradeHandler != nil {
		s.Use(path, route.webSocket.UpgradeHandler)
	}

	// Получаем handler маршрута
	h := route.GetHandler(s)

	// Перебираем параметры адресной строки
	for _, param := range route.Params {
		// Объединяем путь и параметры
		path = p.Join(path, param)
		// Добавляем метод, путь и обработчик
		s.Add(method, path, h)
		// Добавляем запись в swagger
		s.Swagger.Add(name, path, route)
	}
}

// RegisterExt Регистрация интерфейсов
func (s *Server) RegisterExt(v interface{}, path string, name string, suffix ...Suffix) *Server {
	//Устанавливаем имя и путь
	name, path = s.getPkgNameAndPath(path, name, v, suffix...)
	// Проверка интерфейса на соответствие
	switch i := v.(type) {
	case IRestOptions:
		s.restOptions(i, name, path)
		break
	case IRest:
		s.rest(i, name, path)
		break
	case IWeb:
		s.web(i, name, path)
		break
	case IGet:
		s.get(i, name, path)
		break
	case IPost:
		s.post(i, name, path)
		break
	case IPut:
		s.put(i, name, path)
		break
	case IDelete:
		s.delete(i, name, path)
		break
	}
	if s.Config.NotFoundPage != "" {
		s.Use(func(ctx *fiber.Ctx) error {
			return ctx.Render(s.Config.NotFoundPage, nil)
		})
	}
	return s
}

func (s *Server) Register(i interface{}, path string) *Server {
	return s.RegisterExt(i, path, "")
}

// SetCors Установка CORS
func (s *Server) SetCors(config *Cors) *Server {
	cfg := cors.ConfigDefault
	if config != nil {
		cfg = cors.Config(*config)
	}
	s.Use(cors.New(cfg))
	return s
}

// Stop Остановка сервера
func (s *Server) Stop() error {
	s.IsStarted = false
	return s.Shutdown()
}

//Ищем все после пакета controllers
func (s *Server) getPkgNameAndPath(path, name string, v interface{}, suffix ...Suffix) (string, string) {
	//Если имя и путь установлены вручную, то выходим
	if path != "" && name != "" {
		return name, path
	}
	//Извлекаем имя и путь до controllers
	var t reflect.Type
	value := reflect.ValueOf(v)
	if value.Type().Kind() == reflect.Ptr {
		t = reflect.Indirect(value).Type()
	} else {
		t = value.Type()
	}
	pkg := strings.Replace(
		regexp.MustCompile(`controllers(.*)$`).FindString(t.PkgPath()),
		"controllers",
		"",
		-1,
	)
	if name == "" {
		name = strings.Title(t.Name())
	}

	if path == "" {
		array := strings.Split(pkg, "/")
		for _, item := range suffix {
			array = s.insert(array, item.Index, item.Value)
		}
		path = strings.Join(array, "/") + "/" + strings.ToLower(name)
	}

	return strings.Title(name), path
}

func (s *Server) insert(a []string, index int, value string) []string {
	if len(a) == index { // nil or empty slice or after last element
		return append(a, value)
	} else if len(a) < index {
		return a
	}
	a = append(a[:index+1], a[index:]...) // index < len(a)
	a[index] = value
	return a
}
