systemd_system_unit_dir = systemd.get_variable(pkgconfig: 'systemdsystemunitdir')

install_data('rhc-canonical-facts.service', install_dir: systemd_system_unit_dir)
install_data('rhc-canonical-facts.timer', install_dir: systemd_system_unit_dir)

if get_option('rhcd_compatibility')
  install_data(
    'rhcd.conf',
    install_dir: join_paths(systemd_system_unit_dir, 'yggdrasil.service.d'),
  )
endif
