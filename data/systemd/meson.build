systemd_system_unit_dir = systemd.get_variable(pkgconfig: 'systemdsystemunitdir')

install_data('rhc-canonical-facts.service', install_dir: systemd_system_unit_dir)
install_data('rhc-canonical-facts.timer', install_dir: systemd_system_unit_dir)
