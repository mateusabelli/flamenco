package job_compilers

// SPDX-License-Identifier: GPL-3.0-or-later

import (
	"github.com/dop251/goja"
	"github.com/rs/zerolog/log"
	"projects.blender.org/studio/flamenco/pkg/crosspath"
)

// PathModule provides file path manipulation functions by wrapping Go's `path`.
func PathModule(r *goja.Runtime, module *goja.Object) {
	obj := module.Get("exports").(*goja.Object)

	mustExport := func(name string, value any) {
		err := obj.Set(name, value)
		if err != nil {
			log.Panic().Err(err).Msgf("unable to register '%s' in Goja 'path' module", name)
		}
	}

	mustExport("basename", crosspath.Base)
	mustExport("dirname", crosspath.Dir)
	mustExport("join", crosspath.Join)
	mustExport("stem", crosspath.Stem)
}
