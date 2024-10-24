systemd_system_unit_dir = systemd.get_variable(pkgconfig: 'systemdsystemunitdir')
systemd_system_preset_dir = systemd.get_variable(pkgconfig: 'systemdsystempresetdir')

install_data('80-rhc.preset', install_dir: systemd_system_preset_dir)
install_data('rhc-canonical-facts.service', install_dir: systemd_system_unit_dir)
install_data('rhc-canonical-facts.timer', install_dir: systemd_system_unit_dir)
