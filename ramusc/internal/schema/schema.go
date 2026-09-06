// Пакет schema хранит JSON Schema модели — единственный источник истины по
// форме документа (Р10). Отсюда же схему можно отдать наружу: с constrained
// decoding нейросеть физически не может выдать структурно невалидный документ.
package schema

import (
	"bytes"
	_ "embed"
	"fmt"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

//go:embed model.schema.json
var Source []byte

// URL — идентификатор схемы; он же `$id` внутри файла.
const URL = "https://github.com/Zdertis420/autoramus/schema/model.schema.json"

var compiled = sync.OnceValues(func() (*jsonschema.Schema, error) {
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(Source))
	if err != nil {
		return nil, fmt.Errorf("разбор встроенной схемы: %w", err)
	}
	c := jsonschema.NewCompiler()
	if err := c.AddResource(URL, doc); err != nil {
		return nil, fmt.Errorf("регистрация встроенной схемы: %w", err)
	}
	sch, err := c.Compile(URL)
	if err != nil {
		return nil, fmt.Errorf("компиляция встроенной схемы: %w", err)
	}
	return sch, nil
})

// Model отдаёт скомпилированную схему. Ошибка здесь означает, что сломан
// встроенный файл, то есть внутреннюю ошибку компилятора, а не входные данные.
func Model() (*jsonschema.Schema, error) { return compiled() }
