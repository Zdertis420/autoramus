package rsf

// Туннельные концы: где Ramus нарисует скобки.
//
// Признак туннеля в файле не записан. Поле TUNNEL_SOFT, на которое падает
// подозрение, равно нулю во всех проверенных файлах, включая записанные самим
// Ramus, — оно говорит лишь о виде скобок, а не об их наличии. Ramus решает
// иначе (AbstractCrosspoint.getTunnelType, PaintSector.get{Start,End}TunnelType):
//
//	if (isDLevel()) {
//	    if (ins.length > 0 && outs.length > 0) return TUNNEL_NONE;
//	    return sbp.getTunnelSoft();
//	}
//	return TUNNEL_NONE;
//
// isDLevel() истинно, когда ins или outs пуст, поэтому условие сводится к
// одному: скобки рисуются там, где у узла нет входов либо нет выходов. Поверх
// этого PaintSector снимает два случая — конец на краю контекстной диаграммы и
// конец на блоке без декомпозиции: и там, и там продолжаться некуда.

// Tunnel — конец сектора, который Ramus нарисует в скобках.
type Tunnel struct {
	Sector     int64 // элемент сектора
	Start      bool  // начало сектора, а не конец
	Crosspoint int64 // номер узла, у которого не хватает половины
	Diagram    int64 // на чьей диаграмме нарисован сегмент
	Stream     int64 // какой поток несёт
	Ins, Outs  int   // наполнение узла: ради него всё и считается
}

// Tunnels отдаёт концы секторов, которые Ramus нарисует туннельными.
//
// Порядок — порядок секторов, внутри сектора начало раньше конца: обход
// отображения дал бы разный ответ от запуска к запуску.
//
// Функция не судит, правилен туннель или нет. Скобки у намеренно
// туннелированной стрелки и у потерянной выглядят одинаково, и отличить их
// можно только по документу — это работа валидатора, а не разбора файла.
func (m *Model) Tunnels() []Tunnel {
	sectors := m.Sectors()

	// Наполнение узлов. Начало сектора выходит из узла, конец входит в него:
	// так их раскладывает сам Ramus (NDataPlugin.crosspointsListener).
	ins := make(map[int64]int, len(sectors))
	outs := make(map[int64]int, len(sectors))
	for _, s := range sectors {
		if s.Start != nil && s.Start.Crosspoint >= 0 {
			outs[s.Start.Crosspoint]++
		}
		if s.End != nil && s.End.Crosspoint >= 0 {
			ins[s.End.Crosspoint]++
		}
	}

	decomposed := m.decomposed()

	var out []Tunnel
	for _, s := range sectors {
		for _, side := range []struct {
			border *Border
			start  bool
		}{{s.Start, true}, {s.End, false}} {
			if !m.tunnelled(s, side.border, ins, outs, decomposed) {
				continue
			}
			out = append(out, Tunnel{
				Sector:     s.ID,
				Start:      side.start,
				Crosspoint: side.border.Crosspoint,
				Diagram:    s.Diagram,
				Stream:     s.Stream,
				Ins:        ins[side.border.Crosspoint],
				Outs:       outs[side.border.Crosspoint],
			})
		}
	}
	return out
}

// tunnelled решает про один конец.
func (m *Model) tunnelled(s Sector, b *Border, ins, outs map[int64]int, decomposed map[int64]bool) bool {
	if b == nil || b.Crosspoint < 0 {
		// Висящий конец: узла нет, и рисовать скобки не у чего.
		return false
	}
	if ins[b.Crosspoint] > 0 && outs[b.Crosspoint] > 0 {
		// Узел полон: стрелка продолжается, туннеля нет.
		return false
	}
	if b.OnBorder() && s.Diagram == m.Element {
		// Край контекстной диаграммы: выше уровня нет, продолжаться некуда.
		return false
	}
	if b.OnFunction() && !decomposed[b.Function] {
		// Блок-лист: вглубь идти тоже некуда.
		return false
	}
	return true
}

// decomposed отмечает работы, у которых есть хоть один потомок.
func (m *Model) decomposed() map[int64]bool {
	out := make(map[int64]bool)
	for _, f := range m.Functions() {
		if f.Parent >= 0 {
			out[f.Parent] = true
		}
	}
	return out
}
