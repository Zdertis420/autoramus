package layout

import "github.com/Zdertis420/autoramus/ramusc/internal/ir"

// Порядок блоков на диаграмме: сначала те, чей выход питает остальных.
// Топологическая сортировка по потокам; при равенстве — порядок объявления,
// он обычно осмыслен (Р3).

// edges строит рёбра «работа → работа» для одной диаграммы.
//
// В IR связи лежат половинками: у записи из in/control/mechanism заполнено
// только To, у записи из out — только From (ir.Build). Готовых рёбер там нет, и
// пары приходится собирать самим: ребро появляется, когда на этой же диаграмме
// одна работа производит поток, а другая его потребляет.
//
// Поток, приходящий с границы листа, ребра не даёт: его тут никто не
// производит.
func edges(m *ir.Model, siblings map[string]bool) map[string][]string {
	// Кто производит поток. Одну и ту же вещь могут производить несколько
	// работ — берутся все.
	producers := make(map[string][]string)
	for _, l := range m.Links {
		if l.From.Name == "" || !siblings[l.From.Name] {
			continue
		}
		producers[l.Flow.Name] = append(producers[l.Flow.Name], l.From.Name)
	}

	out := make(map[string][]string)
	for _, l := range m.Links {
		if l.To.Name == "" || !siblings[l.To.Name] {
			continue
		}
		for _, from := range producers[l.Flow.Name] {
			if from != l.To.Name {
				out[from] = append(out[from], l.To.Name)
			}
		}
	}
	return out
}

// sort раскладывает работы диаграммы в порядке потоков.
//
// order задаёт порядок объявления и служит разрешением ничьих: при прочих
// равных вперёд идёт объявленный раньше.
//
// Обратные рёбра не мешают: цикл в потоках — это законная стрелка обратной
// связи IDEF0, и валидатор её не запрещает (запрещены только циклы в иерархии
// работ). Работа, у которой остались только непройденные входящие рёбра из
// цикла, выпускается по порядку объявления.
func sortByFlow(order []string, next map[string][]string) []string {
	incoming := make(map[string]int, len(order))
	for _, name := range order {
		incoming[name] = 0
	}
	for from, tos := range next {
		if _, ok := incoming[from]; !ok {
			continue
		}
		for _, to := range tos {
			if _, ok := incoming[to]; ok {
				incoming[to]++
			}
		}
	}

	placed := make(map[string]bool, len(order))
	out := make([]string, 0, len(order))

	for len(out) < len(order) {
		// Обход идёт по order, а не по отображению: порядок объявления
		// разрешает ничьи, и от случайного обхода map результат зависеть не
		// должен (принцип III конституции).
		taken := ""
		for _, name := range order {
			if !placed[name] && incoming[name] == 0 {
				taken = name
				break
			}
		}
		if taken == "" {
			// Остались только работы, связанные циклом. Обратное ребро
			// разрывается: берётся первая по объявлению.
			for _, name := range order {
				if !placed[name] {
					taken = name
					break
				}
			}
		}

		placed[taken] = true
		out = append(out, taken)
		for _, to := range next[taken] {
			if !placed[to] && incoming[to] > 0 {
				incoming[to]--
			}
		}
	}
	return out
}
