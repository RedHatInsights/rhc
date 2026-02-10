project('rhc', version : '0.3.8', meson_version : '>= 0.56.0')

go = find_program('go')
systemd = dependency('systemd', version: '>=239')
bash_completion = dependency('bash-completion')

if get_option('vendor')
  meson.add_dist_script(join_paths('scripts', 'vendor.sh'))
endif

goldflags = get_option('goldflags')
goldflags += ' -X "main.Version=' + meson.project_version() + '"'
goldflags += ' -X "main.LogDir=/var/log/rhc/"'

gobuildflags = get_option('gobuildflags')

subdir('cmd/rhc')
subdir('data')
subdir('dist')
subdir('doc')

summary(
  {
    'vendor': get_option('vendor'),
  },
  section: 'Options',
)
