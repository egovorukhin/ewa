package egowebapi

// Permission структура описывает разрешения на запрос
type Permission struct {
	AllRoutes            bool
	Handler              PermissionHandler
	NotPermissionHandler ErrorHandler
}

// check Проверяем запрос на разрешения
func (p *Permission) check(id interface{}, path string) bool {
	if p.Handler == nil {
		return true
	}
	return p.Handler(id, path)
}
