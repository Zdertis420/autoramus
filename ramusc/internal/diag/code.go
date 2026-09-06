package diag

// Code — устойчивый идентификатор вида ошибки. Список закрытый: GUI ветвится
// по коду, поэтому значения не переименовываются задним числом, а только
// добавляются.
type Code string

// Разбор документа.
const (
	// CodeEmptyDocument — документ пуст или состоит из пробелов.
	CodeEmptyDocument Code = "empty_document"
	// CodeSyntax — синтаксическая ошибка JSON или YAML.
	CodeSyntax Code = "syntax"
	// CodeDuplicateKey — ключ объекта объявлен дважды (Р15).
	CodeDuplicateKey Code = "duplicate_key"
	// CodeYAMLAnchor — якорь или алиас YAML; запрещены (Р15).
	CodeYAMLAnchor Code = "yaml_anchor"
	// CodeYAMLTag — тег YAML, не имеющий соответствия в модели данных JSON.
	CodeYAMLTag Code = "yaml_tag"
)

// Валидация по схеме. Коды повторяют ключевые слова JSON Schema, чтобы
// сообщение и причина не расходились.
const (
	CodeSchema               Code = "schema"
	CodeSchemaType           Code = "schema_type"
	CodeSchemaRequired       Code = "schema_required"
	CodeSchemaUnknownField   Code = "schema_unknown_field"
	CodeSchemaMinItems       Code = "schema_min_items"
	CodeSchemaMaxItems       Code = "schema_max_items"
	CodeSchemaMinLength      Code = "schema_min_length"
	CodeSchemaMaxLength      Code = "schema_max_length"
	CodeSchemaPattern        Code = "schema_pattern"
	CodeSchemaEnum           Code = "schema_enum"
	CodeSchemaConst          Code = "schema_const"
	CodeSchemaUniqueItems    Code = "schema_unique_items"
	CodeSchemaMinProperties  Code = "schema_min_properties"
	CodeSchemaMaxProperties  Code = "schema_max_properties"
	CodeSchemaPropertyNames  Code = "schema_property_names"
	CodeSchemaNotAllowedHere Code = "schema_not_allowed_here"
)
