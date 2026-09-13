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
//
// Часть кодов помечена ниже как «схемой не используется». Такой код объявлен и
// переводится из ошибки библиотеки, но возникнуть не может: model.schema.json
// соответствующего ключевого слова не содержит. Удалять их не стали — тогда
// первое же ужесточение схемы дало бы вместо внятного сообщения общее, — но и
// требовать на них проверку нельзя: входа, на котором они срабатывают, не
// существует. Пометку снимает тот, кто добавит ключевое слово в схему; после
// этого проверка покрытия потребует модель (FR-008).
const (
	// CodeSchema — причина не сводится к частному ключевому слову.
	// Схемой не используется: все виды ошибок нашей схемы разобраны поимённо.
	CodeSchema             Code = "schema"
	CodeSchemaType         Code = "schema_type"
	CodeSchemaRequired     Code = "schema_required"
	CodeSchemaUnknownField Code = "schema_unknown_field"
	CodeSchemaMinItems     Code = "schema_min_items"
	CodeSchemaMaxItems     Code = "schema_max_items"
	CodeSchemaMinLength    Code = "schema_min_length"
	// Схемой не используется: ограничений maxLength в model.schema.json нет.
	CodeSchemaMaxLength Code = "schema_max_length"
	// Схемой не используется: образцов pattern в model.schema.json нет.
	CodeSchemaPattern Code = "schema_pattern"
	CodeSchemaEnum    Code = "schema_enum"
	// Схемой не используется: ограничений const в model.schema.json нет.
	CodeSchemaConst       Code = "schema_const"
	CodeSchemaUniqueItems Code = "schema_unique_items"
	// Схемой не используется: ограничений minProperties в схеме нет.
	CodeSchemaMinProperties Code = "schema_min_properties"
	// Схемой не используется: ограничений maxProperties в схеме нет.
	CodeSchemaMaxProperties Code = "schema_max_properties"
	// Схемой не используется: ограничений propertyNames в схеме нет.
	CodeSchemaPropertyNames Code = "schema_property_names"
	// Схемой не используется: запрещающих подсхем (false) в схеме нет.
	CodeSchemaNotAllowedHere Code = "schema_not_allowed_here"
)

// Смысл модели (Р13). Эти проверки идут после схемы и знают уже не про форму
// документа, а про методологию: объявлен ли поток, сходится ли декомпозиция.
const (
	// CodeEmptyName — имя состоит из одних пробелов (Р4).
	CodeEmptyName Code = "empty_name"
	// CodeDuplicateFunction — две работы с одним именем; имя и есть идентификатор.
	CodeDuplicateFunction Code = "duplicate_function"
	// CodeDuplicateFlow — поток объявлен в flows дважды.
	CodeDuplicateFlow Code = "duplicate_flow"
	// CodeDuplicateStream — атрибуты одного потока заданы дважды.
	CodeDuplicateStream Code = "duplicate_stream"
	// CodeUnknownFlow — имя потока не объявлено в flows (Р6).
	CodeUnknownFlow Code = "unknown_flow"
	// CodeUnknownFunction — ссылка на несуществующую работу.
	CodeUnknownFunction Code = "unknown_function"
	// CodeUnknownClassifier — ссылка на несуществующий классификатор.
	CodeUnknownClassifier Code = "unknown_classifier"
	// CodeHierarchyCycle — работа оказывается собственным предком.
	CodeHierarchyCycle Code = "hierarchy_cycle"
	// CodeNoRoot — нет работы без of, то есть нет контекстной диаграммы.
	CodeNoRoot Code = "no_root"
	// CodeMultipleRoots — работ без of больше одной.
	CodeMultipleRoots Code = "multiple_roots"
	// CodeRootNameMismatch — имя корневой работы не совпадает с полем model.
	CodeRootNameMismatch Code = "root_name_mismatch"
	// CodeNoInput — у работы IDEF0 нет ни одного входа.
	CodeNoInput Code = "no_input"
	// CodeNoControl — у работы IDEF0 нет управления.
	CodeNoControl Code = "no_control"
	// CodeNoMechanism — у работы IDEF0 нет механизма.
	CodeNoMechanism Code = "no_mechanism"
	// CodeNoOutput — у работы нет ни одного выхода.
	CodeNoOutput Code = "no_output"
	// CodeFlowNotProduced — поток потребляется, но на этой диаграмме его никто
	// не производит и от родителя он не приходит.
	CodeFlowNotProduced Code = "flow_not_produced"
	// CodeFlowNotDecomposed — стрелка работы не отражена на её декомпозиции.
	CodeFlowNotDecomposed Code = "flow_not_decomposed"
	// CodeFlowSideMismatch — граничная стрелка меняет сторону ICOM при переходе
	// с родительской диаграммы на дочернюю.
	CodeFlowSideMismatch Code = "flow_side_mismatch"
	// CodeUnusedFlow — поток объявлен, но нигде не используется. Предупреждение.
	CodeUnusedFlow Code = "unused_flow"
	// CodeLinkEndpointsMissing — у связи нет ни from, ни to.
	CodeLinkEndpointsMissing Code = "link_endpoints_missing"
	// CodeLinkNotSiblings — концы связи лежат на разных диаграммах.
	CodeLinkNotSiblings Code = "link_not_siblings"
	// CodeDFDTypeOutsideDFD — тип элемента DFD у работы обычной диаграммы.
	CodeDFDTypeOutsideDFD Code = "dfd_type_outside_dfd"
	// CodeDuplicateClassifier — два классификатора с одним именем.
	CodeDuplicateClassifier Code = "duplicate_classifier"
	// CodeDuplicateColumn — две колонки с одним именем.
	CodeDuplicateColumn Code = "duplicate_column"
	// CodeUnknownColumn — ячейка называет колонку, которой нет.
	CodeUnknownColumn Code = "unknown_column"
	// CodeDuplicateCell — в строке две ячейки на одну колонку.
	CodeDuplicateCell Code = "duplicate_cell"
	// CodeCellType — значение не подходит типу колонки.
	CodeCellType Code = "cell_type"
	// CodeUnknownRow — ссылка на строку классификатора, которой нет.
	CodeUnknownRow Code = "unknown_row"
	// CodeUnexpectedColumnTarget — поле of у колонки, которая никуда не ссылается.
	CodeUnexpectedColumnTarget Code = "unexpected_column_target"
	// CodeDuplicateLayout — две записи раскладки на одну работу.
	CodeDuplicateLayout Code = "duplicate_layout"
	// CodeBadGeometry — координата или размер, которых не бывает.
	CodeBadGeometry Code = "bad_geometry"
	// CodeBadEndpoint — конец сегмента стрелки описан неверно.
	CodeBadEndpoint Code = "bad_endpoint"
	// CodeLonelyNode — узел стрелки упомянут один раз и ничего не сшивает.
	CodeLonelyNode Code = "lonely_node"
	// CodeIncompleteArrowLayout — автор нарисовал поток на диаграмме, но не все
	// его связи там: непокрытая связь в файл не поедет.
	CodeIncompleteArrowLayout Code = "incomplete_arrow_layout"
	// CodeDuplicateLink — связь записана дважды: и списками ICOM, и в links.
	// Предупреждение: в файл всё равно поедет одна стрелка.
	CodeDuplicateLink Code = "duplicate_link"
	// CodeFlowEscapesDiagram — поток произведён на диаграмме, никем там не
	// потреблён и уходит за край, но у работы-владельца такого выхода нет.
	CodeFlowEscapesDiagram Code = "flow_escapes_diagram"
)
