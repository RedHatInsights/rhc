project('rhc', version: '0.3.0', meson_version: '>= 0.56.0')

go = find_program('go')
systemd = dependency('systemd', version: '>=239')
bash_completion = dependency('bash-completion')

if get_option('vendor')
  meson.add_dist_script(join_paths('scripts', 'vendor.sh'))
endif

goldflags = get_option('goldflags')
goldflags += ' -X "main.Version=' + meson.project_version() + '"'
goldflags += ' -X "main.ShortName=rhc"'
goldflags += ' -X "main.LongName=rhc"'
goldflags += ' -X "main.PrefixDir=' + get_option('prefix') + '"'
goldflags += ' -X "main.BinDir=' + get_option('bindir') + '"'
goldflags += ' -X "main.SbinDir=' + get_option('sbindir') + '"'
goldflags += ' -X "main.LibexecDir=' + get_option('libexecdir') + '"'
goldflags += ' -X "main.SysconfDir=' + get_option('sysconfdir') + '"'
goldflags += ' -X "main.DataDir=' + get_option('datadir') + '"'
goldflags += ' -X "main.DatarootDir=' + get_option('datadir') + '"'
goldflags += ' -X "main.ManDir=' + get_option('mandir') + '"'
goldflags += ' -X "main.DocDir=' + get_option('prefix') / 'doc' + '"'
goldflags += ' -X "main.LocalstateDir=' + get_option('localstatedir') + '"'
goldflags += ' -X "main.ServiceName=yggdrasil"'

gobuildflags = get_option('gobuildflags')

rhc = custom_target(
  'rhc',
  build_always_stale: true,
  output: 'rhc',
  command: [
    go,
    'build',
    gobuildflags,
    '-o', '@OUTPUT@',
    '-ldflags', goldflags,
    'github.com/redhatinsights/rhc',
  ],
  install: true,
  install_dir: get_option('bindir'),
)

subdir('data')
subdir('dist')
subdir('doc')

summary(
  {
    'vendor': get_option('vendor'),
  },
  section: 'Options',
)
