rhc = custom_target('rhc',
  build_always_stale: true,
  output: 'rhc',
  command: [go, 'build', gobuildflags, '-o', '@OUTPUT@', '-ldflags', goldflags, 'github.com/redhatinsights/rhc/cmd/rhc'],
  install: true,
  install_dir: get_option('bindir')
)
