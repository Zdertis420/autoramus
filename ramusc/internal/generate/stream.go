package generate

import (
	"fmt"

	"github.com/Zdertis420/autoramus/ramusc/internal/ir"
	"github.com/Zdertis420/autoramus/ramusc/internal/rsf"
)

// writeStreams записывает потоки: элемент, место в списке и имя.
//
// Поток устроен проще работы — три строки против десяти, — но имя лежит в
// своём атрибуте F_STREAM_NAME, а не там же, где у работы. Перепутать их
// значило бы получить поток без подписи и не понять почему.
//
// Пишутся только потоки, у которых есть хоть одна нарисованная стрелка
// (FR-001). Объявленный, но никем не использованный поток — это предупреждение
// валидатора, и заводить под него элемент файла незачем.
func writeStreams(m *rsf.Model, source *ir.Model) (map[string]int64, error) {
	drawn := make(map[string]bool)
	if source.Layout != nil {
		for _, a := range source.Layout.Arrows {
			drawn[a.Flow.Name] = true
		}
	}

	qualifier, ok := m.Qualifier("F_STREAMS")
	if !ok {
		return nil, fmt.Errorf("в заготовке нет квалификатора F_STREAMS")
	}
	hierarchical, ok := m.Attribute("HierarchicalAttribute")
	if !ok {
		return nil, fmt.Errorf("в заготовке нет атрибута HierarchicalAttribute")
	}
	name, ok := m.Attribute("F_STREAM_NAME")
	if !ok {
		return nil, fmt.Errorf("в заготовке нет атрибута F_STREAM_NAME")
	}

	next, err := m.File.NextID("elements", "ELEMENT_ID")
	if err != nil {
		return nil, err
	}

	// Потоки идут цепочкой: каждый помнит предыдущего. Родителя у них нет —
	// в настоящих файлах PARENT_ELEMENT_ID у всех потоков равен -1.
	previous := int64(-1)
	ids := make(map[string]int64, len(source.Flows))

	for _, flow := range source.Flows {
		if !drawn[flow.Name] {
			continue
		}
		id := next
		next++

		element := fmt.Sprint(id)
		rows := []struct {
			table  string
			values map[string]string
		}{
			{"elements", map[string]string{
				"ELEMENT_ID":        element,
				"ELEMENT_NAME":      "",
				"QUALIFIER_ID":      fmt.Sprint(qualifier),
				"CREATED_BRANCH_ID": "0",
				"REMOVED_BRANCH_ID": rsf.AliveBranch,
			}},
			{"attribute_hierarchicals", map[string]string{
				"ATTRIBUTE_ID":        fmt.Sprint(hierarchical),
				"ELEMENT_ID":          element,
				"ICON_ID":             "-1",
				"PARENT_ELEMENT_ID":   "-1",
				"PREVIOUS_ELEMENT_ID": fmt.Sprint(previous),
				"VALUE_BRANCH_ID":     "0",
			}},
			{"attribute_texts", map[string]string{
				"ATTRIBUTE_ID":    fmt.Sprint(name),
				"ELEMENT_ID":      element,
				"VALUE":           flow.Name,
				"VALUE_BRANCH_ID": "0",
			}},
		}

		for _, r := range rows {
			table, err := m.File.Table(r.table)
			if err != nil {
				return nil, err
			}
			if _, err := table.Add(r.values); err != nil {
				return nil, fmt.Errorf("%s: %w", r.table, err)
			}
		}

		ids[flow.Name] = id
		previous = id
	}
	return ids, nil
}
