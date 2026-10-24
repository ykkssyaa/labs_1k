// Package access — роли и права. Ядро и обработчики спрашивают разрешение только здесь.
package access

// Actor — кто выполняет действие.
type Actor struct {
	ID   int64
	Name string
	Role string
}

// Resource — над чем выполняется действие. Для действий без объекта (например,
// taxonomy.manage) достаточно пустого Resource.
type Resource struct {
	Type     string // тип материала, "" — не относится к типу
	AuthorID int64  // владелец материала, 0 — нет владельца
	Status   string // статус материала
}

// Authorizer отвечает на вопрос «можно ли actor выполнить action над res».
type Authorizer interface {
	Can(actor Actor, action string, res Resource) bool
}

// AllowAll разрешает всё. Только для тестов ядра, где права не предмет проверки.
type AllowAll struct{}

func (AllowAll) Can(Actor, string, Resource) bool { return true }
