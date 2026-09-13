// Проверки смысла модели (Р13). Схема отвечает за форму документа, этот слой —
// за методологию: объявлен ли поток, существует ли работа, сходится ли
// декомпозиция. Сообщения здесь адресные и с подсказками, потому что их главный
// читатель — цикл генерация → проверка → исправление вокруг нейросети.
package validate

import (
	"fmt"
	"strings"

	"github.com/Zdertis420/autoramus/ramusc/internal/diag"
	"github.com/Zdertis420/autoramus/ramusc/internal/ir"
)

// Semantic проверяет уже разобранную и прошедшую схему модель.
func Semantic(m *ir.Model) diag.List {
	if m == nil {
		return nil
	}
	c := &checker{
		m:         m,
		flows:     make(map[string]ir.Ref),
		functions: make(map[string]*ir.Function),
		produced:  make(map[string]map[string]bool),
		producers: make(map[string]bool),
		inputs:    make(map[string]map[string]map[string]bool),
		sides:     make(map[string]map[string]bool),
		outputs:   make(map[string]map[string]bool),
		linksTo:   make(map[string][]*ir.Link),
		linksFrom: make(map[string][]*ir.Link),
		used:      make(map[string]bool),
	}

	c.collectFlows()
	c.collectFunctions()
	c.checkHierarchy()
	c.collectLinks()
	c.checkDuplicateLinks()
	c.checkICOM()
	c.checkBalance()
	c.checkDecomposition()
	c.checkTunnels()
	c.checkUnusedFlows()
	c.checkStreams()
	c.checkTypes()
	c.checkClassifiers()
	c.checkLayout()

	c.out.Sort()
	return c.out
}

type checker struct {
	m   *ir.Model
	out diag.List

	flows     map[string]ir.Ref // объявленный поток → первое объявление
	flowNames []string          // кандидаты для подсказок, в порядке документа

	functions map[string]*ir.Function // имя работы → первая с таким именем
	funcNames []string

	classifierNames []string

	// produced: диаграмма → поток → его кто-то выдаёт.
	produced map[string]map[string]bool
	// producers: работа выдаёт хоть что-нибудь.
	producers map[string]bool
	// inputs: работа → поток → сторона.
	inputs map[string]map[string]map[string]bool
	// sides: работа → сторона, на которой к ней хоть что-то приходит.
	sides map[string]map[string]bool
	// outputs: работа → поток, который она выдаёт.
	outputs map[string]map[string]bool
	// Связи, прицепленные к работе, в порядке документа: нужны, чтобы
	// поставить маркер на само имя потока в её ICOM.
	linksTo   map[string][]*ir.Link
	linksFrom map[string][]*ir.Link

	consumers []consumer
	used      map[string]bool // поток встретился хоть в одной связи
}

// consumer — потребление потока работой; хранится для проверки баланса.
type consumer struct {
	fn   *ir.Function
	flow ir.Ref
	side string
}

func (c *checker) add(d diag.Diagnostic) { c.out.Add(d) }

// name проверяет, что имя не пусто после нормализации (Р4).
func (c *checker) name(ref ir.Ref, what string) bool {
	if !ref.Set() {
		return false
	}
	if ref.Name == "" {
		c.add(diag.New(diag.CodeEmptyName, ref.Pos, ref.Path,
			"%s не может быть пустым: в нём одни пробелы", what))
		return false
	}
	return true
}

func (c *checker) collectFlows() {
	for _, f := range c.m.Flows {
		if !c.name(f, "имя потока") {
			continue
		}
		if first, dup := c.flows[f.Name]; dup {
			c.add(diag.New(diag.CodeDuplicateFlow, f.Pos, f.Path,
				"поток «%s» уже объявлен в строке %d", f.Name, first.Pos.Line))
			continue
		}
		c.flows[f.Name] = f
		c.flowNames = append(c.flowNames, f.Name)
	}
}

func (c *checker) collectFunctions() {
	c.name(c.m.Name, "имя модели")
	for _, f := range c.m.Functions {
		if !c.name(f.Name, "имя работы") {
			continue
		}
		if first, dup := c.functions[f.Name.Name]; dup {
			c.add(diag.New(diag.CodeDuplicateFunction, f.Name.Pos, f.Name.Path,
				"работа «%s» уже объявлена в строке %d: имя работы — это её идентификатор (Р4)",
				f.Name.Name, first.Name.Pos.Line))
			continue
		}
		c.functions[f.Name.Name] = f
		c.funcNames = append(c.funcNames, f.Name.Name)
	}
}

// canonical отсеивает работы, о которых уже сказано: безымянные и повторные.
// Иначе на один документ сыпалось бы по три сообщения об одном и том же.
func (c *checker) canonical(f *ir.Function) bool {
	return f.Name.Name != "" && c.functions[f.Name.Name] == f
}

// function разрешает ссылку на работу, ругаясь на неизвестное имя.
func (c *checker) function(ref ir.Ref, what string) *ir.Function {
	if !c.name(ref, what) {
		return nil
	}
	if f, ok := c.functions[ref.Name]; ok {
		return f
	}
	c.add(diag.New(diag.CodeUnknownFunction, ref.Pos, ref.Path,
		"работа «%s» не найдена%s", ref.Name, hint(ref.Name, c.funcNames)))
	return nil
}

// flow разрешает ссылку на поток. Необъявленный поток — ошибка по Р6: без неё
// опечатка тихо порождает лишнюю стрелку.
func (c *checker) flow(ref ir.Ref, what string) bool {
	if !c.name(ref, what) {
		return false
	}
	if _, ok := c.flows[ref.Name]; ok {
		return true
	}
	c.add(diag.New(diag.CodeUnknownFlow, ref.Pos, ref.Path,
		"поток «%s» не объявлен в flows%s", ref.Name, hint(ref.Name, c.flowNames)))
	return false
}

// diagram отдаёт имя диаграммы, на которой живёт работа: пустая строка —
// контекстная диаграмма. Второй результат — false, если родитель не разрешился
// и говорить о диаграмме нельзя.
func (c *checker) diagram(f *ir.Function) (string, bool) {
	if !f.Of.Set() || f.Of.Name == "" {
		return "", true
	}
	p, ok := c.functions[f.Of.Name]
	if !ok {
		return "", false
	}
	return p.Name.Name, true
}

// diagramIn называет диаграмму так, чтобы подставляться после предлога «на».
func diagramIn(diagram string) string {
	if diagram == "" {
		return "контекстной диаграмме"
	}
	return fmt.Sprintf("диаграмме работы «%s»", diagram)
}

// checkHierarchy проверяет ссылки of, единственность корня и отсутствие циклов.
func (c *checker) checkHierarchy() {
	var roots []*ir.Function
	for _, f := range c.m.Functions {
		if !c.canonical(f) {
			continue
		}
		if !f.Of.Set() {
			roots = append(roots, f)
			continue
		}
		c.function(f.Of, "имя родительской работы")
	}

	switch {
	case len(roots) == 0:
		if len(c.functions) > 0 {
			c.add(diag.New(diag.CodeNoRoot, c.m.Name.Pos, c.m.Name.Path,
				"нет ни одной работы без of: контекстную диаграмму не из чего построить"))
		}
	default:
		for _, extra := range roots[1:] {
			c.add(diag.New(diag.CodeMultipleRoots, extra.Name.Pos, extra.Name.Path,
				"работа «%s» тоже без of, а корневая работа должна быть одна (первая — «%s» в строке %d)",
				extra.Name.Name, roots[0].Name.Name, roots[0].Name.Pos.Line))
		}
		if root := roots[0]; c.m.Name.Name != "" && root.Name.Name != c.m.Name.Name {
			c.add(diag.New(diag.CodeRootNameMismatch, root.Name.Pos, root.Name.Path,
				"корневая работа «%s» и поле model «%s» должны совпадать: контекстная диаграмма — это и есть модель",
				root.Name.Name, c.m.Name.Name))
		}
	}

	c.checkCycles()
}

// checkCycles ищет циклы в цепочках of. Граф функциональный — у работы один
// родитель, — поэтому хватает прохода по цепочке с пометками.
func (c *checker) checkCycles() {
	const done = 2
	state := make(map[string]int, len(c.functions))

	for _, start := range c.m.Functions {
		if !c.canonical(start) || state[start.Name.Name] == done {
			continue
		}
		var path []string
		at := make(map[string]int, 4)
		cur := start.Name.Name
		for {
			if state[cur] == done {
				break
			}
			if i, seen := at[cur]; seen {
				c.reportCycle(path[i:])
				break
			}
			at[cur] = len(path)
			path = append(path, cur)

			f := c.functions[cur]
			if f == nil || !f.Of.Set() {
				break
			}
			if _, ok := c.functions[f.Of.Name]; !ok {
				break
			}
			cur = f.Of.Name
		}
		for _, n := range path {
			state[n] = done
		}
	}
}

// reportCycle сообщает о цикле один раз, начиная с работы, которая идёт в
// документе раньше прочих: тогда текст не зависит от порядка обхода.
func (c *checker) reportCycle(cycle []string) {
	if len(cycle) == 0 {
		return
	}
	first := 0
	for i, n := range cycle {
		if c.functions[n].Name.Pos.Line < c.functions[cycle[first]].Name.Pos.Line {
			first = i
		}
	}
	rotated := append(append([]string{}, cycle[first:]...), cycle[:first]...)

	chain := ""
	for _, n := range rotated {
		chain += fmt.Sprintf("«%s» → ", n)
	}
	chain += fmt.Sprintf("«%s»", rotated[0])

	f := c.functions[rotated[0]]
	c.add(diag.New(diag.CodeHierarchyCycle, f.Of.Pos, f.Of.Path,
		"цикл в иерархии работ: %s", chain))
}

// collectLinks проверяет ссылки связей и попутно собирает всё, что нужно
// проверкам выхода и баланса.
func (c *checker) collectLinks() {
	for _, l := range c.m.Links {
		if c.flow(l.Flow, "имя потока") {
			c.used[l.Flow.Name] = true
		}

		var from, to *ir.Function
		if l.Sugar {
			// Конец сахарной связи — сама работа, в которой она написана.
			// Про её имя всё уже сказано там, где оно объявлено.
			from, to = c.functions[l.From.Name], c.functions[l.To.Name]
		} else {
			if l.From.Set() {
				from = c.function(l.From, "имя работы-источника")
			}
			if l.To.Set() {
				to = c.function(l.To, "имя работы-приёмника")
			}
		}

		if !l.Sugar && !l.From.Set() && !l.To.Set() {
			c.add(diag.New(diag.CodeLinkEndpointsMissing, l.Pos, l.Path,
				"у связи нет ни from, ни to: стрелку не из чего построить"))
		}

		if from != nil && to != nil {
			fromDiagram, fromOK := c.diagram(from)
			toDiagram, toOK := c.diagram(to)
			if fromOK && toOK && fromDiagram != toDiagram {
				c.add(diag.New(diag.CodeLinkNotSiblings, l.Pos, l.Path,
					"концы связи лежат на разных диаграммах: «%s» рисуется на %s, «%s» — на %s",
					from.Name.Name, diagramIn(fromDiagram),
					to.Name.Name, diagramIn(toDiagram)))
			}
		}

		if from != nil {
			c.producers[from.Name.Name] = true
			c.linksFrom[from.Name.Name] = append(c.linksFrom[from.Name.Name], l)
			if l.Flow.Name != "" {
				if c.outputs[from.Name.Name] == nil {
					c.outputs[from.Name.Name] = make(map[string]bool)
				}
				c.outputs[from.Name.Name][l.Flow.Name] = true
			}
			if d, ok := c.diagram(from); ok && l.Flow.Name != "" {
				if c.produced[d] == nil {
					c.produced[d] = make(map[string]bool)
				}
				c.produced[d][l.Flow.Name] = true
			}
		}
		if to != nil && l.Flow.Name != "" {
			side := l.SideName()
			c.linksTo[to.Name.Name] = append(c.linksTo[to.Name.Name], l)
			if c.inputs[to.Name.Name] == nil {
				c.inputs[to.Name.Name] = make(map[string]map[string]bool)
			}
			if c.inputs[to.Name.Name][l.Flow.Name] == nil {
				c.inputs[to.Name.Name][l.Flow.Name] = make(map[string]bool)
			}
			c.inputs[to.Name.Name][l.Flow.Name][side] = true
			if c.sides[to.Name.Name] == nil {
				c.sides[to.Name.Name] = make(map[string]bool)
			}
			c.sides[to.Name.Name][side] = true

			if _, declared := c.flows[l.Flow.Name]; declared {
				c.consumers = append(c.consumers, consumer{fn: to, flow: l.Flow, side: side})
			}
		}
	}
}

// checkICOM: у работы IDEF0 есть все четыре стороны — вход, управление,
// механизм и выход, без исключений. Так устроены реальные модели Ramus: в
// `examples/ИзготовлениеЮбки.rsf` каждая работа, включая корневую, несёт полный
// ICOM. Неполный набор означает, что автор просто не дописал стрелки.
//
// Исключения ровно два, и оба — не про IDEF0:
//   - хранилища, внешние сущности и роли DFD: хранилище вполне может быть
//     только приёмником;
//   - элементы на диаграмме DFD: там ICOM нет вовсе, есть только «откуда-куда».
//     Выход при этом спрашивается и с них: процесс, который ничего не выдаёт,
//     бессмыслен в любой нотации.
func (c *checker) checkICOM() {
	for _, f := range c.m.Functions {
		if !c.canonical(f) || f.TypeName() != ir.TypeProcess {
			continue
		}
		if !c.producers[f.Name.Name] {
			c.add(diag.New(diag.CodeNoOutput, f.Name.Pos, f.Name.Path,
				"у работы «%s» нет ни одного выхода: добавьте out или связь из неё",
				f.Name.Name))
		}
		if !c.idef0Work(f) {
			continue
		}
		for _, side := range []struct {
			name string
			code diag.Code
			what string
		}{
			{ir.SideIn, diag.CodeNoInput, "входа"},
			{ir.SideControl, diag.CodeNoControl, "управления"},
			{ir.SideMechanism, diag.CodeNoMechanism, "механизма"},
		} {
			if c.sides[f.Name.Name][side.name] {
				continue
			}
			c.add(diag.New(side.code, f.Name.Pos, f.Name.Path,
				"у работы «%s» нет %s: у работы IDEF0 есть все четыре стороны — "+
					"вход, управление, механизм и выход", f.Name.Name, side.what))
		}
	}
}

// idef0Work отсекает элементы диаграмм DFD: спрашивать с них ICOM не за что.
// Поле kind описывает декомпозицию самой работы, а не её саму, поэтому смотреть
// надо на родителя.
func (c *checker) idef0Work(f *ir.Function) bool {
	if !f.Of.Set() {
		return true // корневая работа контекстной диаграммы
	}
	parent, ok := c.functions[f.Of.Name]
	if !ok {
		return true
	}
	return parent.KindName() != ir.KindDFD
}

// checkBalance проверяет, что декомпозиция сходится с родительской диаграммой.
//
// Потребляемый поток либо производит кто-то на той же диаграмме, либо он
// приходит с границы листа — а граница листа берётся у родителя (Р5), причём
// той же стороной. Если поток не производится и у родителя его нет, стрелке
// неоткуда взяться: ровно так теряются недописанные механизмы и управления.
//
// В IDEF0 есть туннельные стрелки, которых на родительской диаграмме нет, но
// сейчас записать их в языке нечем, и молча считать любой промах туннелем —
// значит пропускать настоящие ошибки. В `examples/ИзготовлениеЮбки.rsf`
// туннелей нет ни одного: каждая граничная стрелка A0 есть и на A-0.
func (c *checker) checkBalance() {
	for _, cons := range c.consumers {
		diagram, ok := c.diagram(cons.fn)
		if !ok || diagram == "" {
			continue
		}
		// Поток родился на этой же диаграмме — это внутренняя стрелка,
		// граница ни при чём.
		if c.produced[diagram][cons.flow.Name] {
			continue
		}
		sides := c.inputs[diagram][cons.flow.Name]
		switch {
		case len(sides) == 0:
			c.add(diag.New(diag.CodeFlowNotProduced, cons.flow.Pos, cons.flow.Path,
				"поток «%s» потребляется, но на диаграмме работы «%s» его никто не производит "+
					"и к самой «%s» он не приходит: добавьте его в %s работы «%s»",
				cons.flow.Name, diagram, diagram, cons.side, diagram))
		case !sides[cons.side]:
			c.add(diag.New(diag.CodeFlowSideMismatch, cons.flow.Pos, cons.flow.Path,
				"работа «%s» принимает поток «%s» стороной %s, а к «%s» он приходит стороной %s: "+
					"у граничной стрелки сторона ICOM не меняется",
				cons.fn.Name.Name, cons.flow.Name, cons.side, diagram, sideList(sides)))
		}
	}

	c.checkEscapes()
}

// checkEscapes — недостающая половина баланса: со стороны производителя.
//
// checkBalance выше идёт от потребителя: поток, который потребляют, должен кто-то
// выдавать. Обратный случай до сих пор не проверялся, а он такой же: поток,
// который на диаграмме произвели и никто не потребил, уходит за край листа — и
// значит обязан быть выходом работы, которой эта диаграмма принадлежит. Иначе
// конец на краю остаётся без пары на родительской диаграмме, и Ramus рисует его
// в скобках. Именно так `skirt-full.yaml` получал туннель, которого автор не
// заказывал.
//
// Контекстная диаграмма исключение: над ней уровня нет, и выход модели уходит
// наружу по определению.
func (c *checker) checkEscapes() {
	for _, f := range c.m.Functions {
		if !c.canonical(f) {
			continue
		}
		diagram, ok := c.diagram(f)
		if !ok || diagram == "" {
			continue // контекстная диаграмма: выше некуда
		}
		owner, ok := c.functions[diagram]
		if !ok {
			continue
		}

		seen := make(map[string]bool)
		for _, l := range c.linksFrom[f.Name.Name] {
			flow := l.Flow.Name
			if flow == "" || seen[flow] {
				continue
			}
			if _, declared := c.flows[flow]; !declared {
				continue // о неизвестном потоке уже сказано
			}
			seen[flow] = true

			if c.consumedOn(diagram, flow) || c.outputs[diagram][flow] {
				continue
			}
			if c.tunnelled(owner, flow) {
				continue // автор сказал: эта стрелка наверх намеренно не идёт
			}
			c.add(diag.New(diag.CodeFlowEscapesDiagram, l.Flow.Pos, l.Flow.Path,
				"поток «%s» выходит из работы «%s», на %s его никто не потребляет, "+
					"и у работы «%s» такого выхода нет: стрелка уйдёт за край листа "+
					"и оборвётся. Добавьте поток в out работы «%s» либо назовите его "+
					"в её tunnel",
				flow, f.Name.Name, diagramIn(diagram), diagram, diagram))
		}
	}
}

// consumedOn сообщает, что поток потребляет хоть одна работа этой диаграммы.
func (c *checker) consumedOn(diagram, flow string) bool {
	for _, f := range c.m.Functions {
		if !c.canonical(f) {
			continue
		}
		if d, ok := c.diagram(f); !ok || d != diagram {
			continue
		}
		if len(c.inputs[f.Name.Name][flow]) > 0 {
			return true
		}
	}
	return false
}

// checkDecomposition — баланс сверху вниз, вторая половина правила.
//
// Всё, что входит в блок и выходит из него, обязано быть видно на его
// декомпозиции: внешние входы, управления и механизмы приходят на границу
// дочерней диаграммы и доходят до блоков, выход собирается с них же. Стрелка,
// которой на декомпозиции нет, просто теряется: в `ИзготовлениеЮбки.rsf`
// у корневого блока отражены все четыре стороны — `Ткань` уходит в раскрой,
// `Фурнитура` в добавление фурнитуры, управление ветвится на все три работы,
// каждый механизм доходит до своей.
//
// Проверяются только работы с декомпозицией: у листа диаграммы нет.
//
// Ветка DFD прежде пропускалась целиком — «там нет ICOM, и переносить
// управления с механизмами не на что». Пропуск оказался дорогим: четыре ICOM
// работы с DFD-декомпозицией упирались в блок, внутри которого продолжения нет,
// и Ramus рисовал у них туннельные скобки. Молча, потому что сказать было
// некому. Теперь спрос одинаков со всех, а намеренный туннель автор объявляет
// полем tunnel — тем и отличается «я так задумал» от «я забыл дописать
// декомпозицию».
func (c *checker) checkDecomposition() {
	children := make(map[string][]*ir.Function, len(c.functions))
	for _, f := range c.m.Functions {
		if !c.canonical(f) || !f.Of.Set() {
			continue
		}
		if parent, ok := c.functions[f.Of.Name]; ok {
			children[parent.Name.Name] = append(children[parent.Name.Name], f)
		}
	}

	for _, p := range c.m.Functions {
		if !c.canonical(p) {
			continue
		}
		kids := children[p.Name.Name]
		if len(kids) == 0 {
			continue
		}

		// Декомпозиция DFD — особый случай, и не потому, что там нет ICOM, а
		// потому, что граничных стрелок на диаграмме потоков данных не
		// рисуется вовсе: цеплять их не к чему — у внешней сущности и
		// хранилища сторон ICOM нет. Значит любая стрелка работы с такой
		// декомпозицией внутрь не идёт и на родительской диаграмме будет в
		// скобках. Это законно, но автор должен сказать об этом сам: забытая
		// декомпозиция выглядела бы ровно так же.
		if p.KindName() == ir.KindDFD {
			c.checkDFDTunnels(p)
			continue
		}

		seen := make(map[string]bool)
		for _, l := range c.linksTo[p.Name.Name] {
			if !c.decomposable(l, seen) || c.tunnelled(p, l.Flow.Name) {
				continue
			}
			// Достаточно, чтобы поток принял хоть кто-то и хоть какой стороной:
			// о перепутанной стороне скажет checkBalance, и второе сообщение
			// об одной и той же стрелке ни к чему.
			if anyChild(kids, func(kid string) bool { return len(c.inputs[kid][l.Flow.Name]) > 0 }) {
				continue
			}
			c.add(diag.New(diag.CodeFlowNotDecomposed, l.Flow.Pos, l.Flow.Path,
				"поток «%s» приходит к работе «%s» стороной %s, но в её декомпозиции "+
					"его никто не принимает: граничная стрелка должна дойти до блока",
				l.Flow.Name, p.Name.Name, l.SideName()))
		}

		seen = make(map[string]bool)
		for _, l := range c.linksFrom[p.Name.Name] {
			if !c.decomposable(l, seen) || c.tunnelled(p, l.Flow.Name) {
				continue
			}
			if anyChild(kids, func(kid string) bool { return c.outputs[kid][l.Flow.Name] }) {
				continue
			}
			c.add(diag.New(diag.CodeFlowNotDecomposed, l.Flow.Pos, l.Flow.Path,
				"поток «%s» выходит из работы «%s», но в её декомпозиции его никто "+
					"не производит: выход собирается с блоков дочерней диаграммы",
				l.Flow.Name, p.Name.Name))
		}
	}
}

// decomposable отсеивает связи, о которых говорить нечего: с неизвестным
// потоком и повторные.
func (c *checker) decomposable(l *ir.Link, seen map[string]bool) bool {
	if l.Flow.Name == "" {
		return false
	}
	if _, declared := c.flows[l.Flow.Name]; !declared {
		return false
	}
	key := l.Flow.Name + "\x00" + l.SideName()
	if seen[key] {
		return false
	}
	seen[key] = true
	return true
}

func anyChild(kids []*ir.Function, ok func(name string) bool) bool {
	for _, kid := range kids {
		if ok(kid.Name.Name) {
			return true
		}
	}
	return false
}

// sideList печатает стороны родителя в устойчивом порядке: множество в Go
// обходится вразнобой, а сообщение должно быть одинаковым от прогона к прогону.
func sideList(sides map[string]bool) string {
	var out []string
	for _, side := range []string{ir.SideIn, ir.SideControl, ir.SideMechanism} {
		if sides[side] {
			out = append(out, side)
		}
	}
	return strings.Join(out, " или ")
}

// checkUnusedFlows: объявленный, но неиспользованный поток — обычно остаток
// от правки. Модель при этом собирается, поэтому предупреждение.
func (c *checker) checkUnusedFlows() {
	for _, name := range c.flowNames {
		if c.used[name] {
			continue
		}
		ref := c.flows[name]
		c.add(diag.NewWarning(diag.CodeUnusedFlow, ref.Pos, ref.Path,
			"поток «%s» объявлен, но нигде не используется", name))
	}
}

func (c *checker) checkStreams() {
	seen := make(map[string]ir.Ref, len(c.m.Streams))
	for _, s := range c.m.Streams {
		if !c.flow(s.Name, "имя потока") {
			continue
		}
		if first, dup := seen[s.Name.Name]; dup {
			c.add(diag.New(diag.CodeDuplicateStream, s.Name.Pos, s.Name.Path,
				"атрибуты потока «%s» уже заданы в строке %d", s.Name.Name, first.Pos.Line))
			continue
		}
		seen[s.Name.Name] = s.Name
	}
}

// checkTypes: store, external и role — элементы DFD, на обычной диаграмме
// IDEF0 им взяться неоткуда (Р8).
func (c *checker) checkTypes() {
	for _, f := range c.m.Functions {
		if !c.canonical(f) || f.TypeName() == ir.TypeProcess {
			continue
		}
		if !f.Of.Set() {
			c.add(diag.New(diag.CodeDFDTypeOutsideDFD, f.Type.Pos, f.Type.Path,
				"тип «%s» допустим только на диаграмме DFD, а «%s» — корневая работа",
				f.TypeName(), f.Name.Name))
			continue
		}
		parent, ok := c.functions[f.Of.Name]
		if !ok {
			continue // о битой ссылке уже сказано
		}
		if parent.KindName() != ir.KindDFD {
			c.add(diag.New(diag.CodeDFDTypeOutsideDFD, f.Type.Pos, f.Type.Path,
				"тип «%s» допустим только на диаграмме DFD, а у работы «%s» не указан kind: dfd",
				f.TypeName(), parent.Name.Name))
		}
	}
}

// checkTunnels проверяет объявления туннелей.
//
// Туннель — утверждение о стрелке, которая уже объявлена в ICOM этой же работы:
// «она намеренно не идёт вглубь». Назвать поток, которого у работы нет, значит
// туннелировать несуществующее — скорее всего опечатка в имени.
func (c *checker) checkTunnels() {
	for _, f := range c.m.Functions {
		if !c.canonical(f) {
			continue
		}
		name := f.Name.Name
		seen := make(map[string]ir.Ref, len(f.Tunnel))
		for _, ref := range f.Tunnel {
			if !c.flow(ref, "имя потока") {
				continue
			}
			c.used[ref.Name] = true

			if first, dup := seen[ref.Name]; dup {
				c.add(diag.New(diag.CodeDuplicateFlow, ref.Pos, ref.Path,
					"поток «%s» назван туннельным дважды: первый раз в строке %d",
					ref.Name, first.Pos.Line))
				continue
			}
			seen[ref.Name] = ref

			if len(c.inputs[name][ref.Name]) == 0 && !c.outputs[name][ref.Name] {
				c.add(diag.New(diag.CodeUnknownFlow, ref.Pos, ref.Path,
					"поток «%s» назван туннельным у работы «%s», но к ней не приходит "+
						"и из неё не выходит: туннелировать нечего",
					ref.Name, name))
			}
		}
	}
}

// checkDFDTunnels требует, чтобы каждая стрелка работы с DFD-декомпозицией
// была объявлена туннельной.
//
// Это не придирка, а описание того, что компилятор умеет: граничных секторов на
// диаграмме потоков данных он не пишет и писать не может — прицепить их не к
// чему, а меры для такой геометрии нет ни в одной модели набора. Значит конец
// на родительской диаграмме останется без пары, и Ramus нарисует скобки.
// Написать `tunnel` — единственный способ сказать, что так и задумано.
func (c *checker) checkDFDTunnels(p *ir.Function) {
	seen := make(map[string]bool)
	for _, links := range [][]*ir.Link{c.linksTo[p.Name.Name], c.linksFrom[p.Name.Name]} {
		for _, l := range links {
			if !c.decomposable(l, seen) || c.tunnelled(p, l.Flow.Name) {
				continue
			}
			c.add(diag.New(diag.CodeFlowNotDecomposed, l.Flow.Pos, l.Flow.Path,
				"поток «%s» связан с работой «%s», а её декомпозиция — диаграмма "+
					"потоков данных: граничные стрелки там не рисуются, и в Ramus "+
					"конец будет в скобках. Если так и задумано, назовите поток "+
					"в tunnel работы «%s»",
				l.Flow.Name, p.Name.Name, p.Name.Name))
		}
	}
}

// tunnelled сообщает, что автор объявил поток туннельным у этой работы.
func (c *checker) tunnelled(f *ir.Function, flow string) bool {
	for _, ref := range f.Tunnel {
		if ref.Name == flow {
			return true
		}
	}
	return false
}
