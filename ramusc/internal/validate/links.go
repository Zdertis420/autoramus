package validate

import (
	"github.com/Zdertis420/autoramus/ramusc/internal/diag"
	"github.com/Zdertis420/autoramus/ramusc/internal/ir"
)

// checkDuplicateLinks ловит связь, записанную дважды.
//
// Одно и то же выразимо двумя способами (Р7): списками ICOM у обеих работ —
// выход у источника, вход (управление, механизм) у приёмника — и явной записью
// в links. Написать оба способа не запрещено и даже соблазнительно: списки
// читаются, а links объясняет. Но в файл при этом ехали две стрелки: половинки
// ICOM и явная запись приходят в раскладку разными ветками.
//
// Выглядело это так, будто дубли наводит сам Ramus: две линии ложились рядом,
// подпись двоилась, а удаление одной оставляло близнеца — у обеих были общие
// номера узлов, и Ramus считал их одним узлом с двумя секторами.
//
// Предупреждение, а не ошибка: файл с такой записью собрать можно и нужно,
// стрелка в нём будет одна. Сказать всё равно надо — вторая запись не делает
// ничего, и автор вправе об этом знать.
func (c *checker) checkDuplicateLinks() {
	// Что уже сказано списками ICOM.
	produced := make(map[string]map[string]bool)            // поток → работа
	consumed := make(map[string]map[string]map[string]bool) // поток → работа → сторона
	for _, l := range c.m.Links {
		if !l.Sugar || l.Flow.Name == "" {
			continue
		}
		if l.From.Name != "" {
			if produced[l.Flow.Name] == nil {
				produced[l.Flow.Name] = make(map[string]bool)
			}
			produced[l.Flow.Name][l.From.Name] = true
		}
		if l.To.Name != "" {
			if consumed[l.Flow.Name] == nil {
				consumed[l.Flow.Name] = make(map[string]map[string]bool)
			}
			if consumed[l.Flow.Name][l.To.Name] == nil {
				consumed[l.Flow.Name][l.To.Name] = make(map[string]bool)
			}
			consumed[l.Flow.Name][l.To.Name][l.SideName()] = true
		}
	}

	// Ключ повтора — то же, чем раскладка схлопывает стрелки: поток, оба конца
	// и сторона приёмника. Разная сторона даёт разный ключ, и две такие связи
	// повтором не считаются: это две разные стрелки, и рисуются они обе.
	type key struct{ flow, from, to, side string }
	seen := make(map[key]ir.Ref)

	for _, l := range c.m.Links {
		if l.Sugar || l.Flow.Name == "" || l.From.Name == "" || l.To.Name == "" {
			continue
		}
		k := key{l.Flow.Name, l.From.Name, l.To.Name, l.SideName()}

		if first, dup := seen[k]; dup {
			c.add(diag.NewWarning(diag.CodeDuplicateLink, l.Flow.Pos, l.Flow.Path,
				"связь «%s» из «%s» в «%s» стороной %s уже записана в строке %d: "+
					"в файл поедет одна стрелка",
				l.Flow.Name, l.From.Name, l.To.Name, l.SideName(), first.Pos.Line))
			continue
		}
		seen[k] = l.Flow

		if produced[l.Flow.Name][l.From.Name] && consumed[l.Flow.Name][l.To.Name][l.SideName()] {
			c.add(diag.NewWarning(diag.CodeDuplicateLink, l.Flow.Pos, l.Flow.Path,
				"связь «%s» из «%s» в «%s» стороной %s уже выражена списками "+
					"%s и out этих работ: запись в links ничего не добавляет, "+
					"в файл поедет одна стрелка",
				l.Flow.Name, l.From.Name, l.To.Name, l.SideName(), l.SideName()))
		}
	}
}
