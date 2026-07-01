# Run gocheck unless explicitly disabled.
%bcond_without check

%if 0%{?rhel} || 0%{?centos}
# Enable rhcd->yggdrasil compatibility scriptlets and files.
# RHEL/CentOS Stream shipped rhc with yggdrasil as rhcd and need migration logic.
%global with_rhcd_compat 1
%endif

%global goipath         github.com/redhatinsights/rhc
Version:                0.3.11

%gometa

Name:           rhc
Release:        %autorelease
Epoch:          1
Summary:        Tool for registering to Red Hat services
License:        Apache-2.0 AND BSD-2-Clause AND BSD-3-Clause AND GPL-3.0-only AND MIT
URL:            %{gourl}
Source0:        %{gosource}
Source1:        %{archivename}-vendor.tar.bz2
%if 0%{?fedora}
Source2:        go-vendor-tools.toml
%endif

BuildRequires:  systemd-rpm-macros

%if 0%{?with_rhcd_compat}
# semanage is called in %post/%postun to manage the rhcd_t SELinux type.
Requires(post): policycoreutils-python-utils
%endif

%if 0%{?fedora}
# Fedora defines its own dependency vendorization policy, different from RHEL.
# These build-time dependencies only exist for Fedora.
BuildRequires:  go-vendor-tools
BuildRequires:  askalono-cli
%endif

%if %{with check}
# rhc's unit test triggered by gocheck require D-Bus.
BuildRequires:  /usr/bin/dbus-launch
%endif

Requires: subscription-manager
Requires: yggdrasil >= 0.4
%if 0%{?rhel}
# insights-client and yggdrasil-worker-package-manager are only available on RHEL <= 10
Requires: insights-client
Requires: yggdrasil-worker-package-manager
%endif

%description
Client tool to register Fedora, CentOS Stream or Red Hat Enterprise Linux
to Red Hat Subscription Management and Red Hat Lightspeed.

%prep
# Unpack Source0 and set up the Go build directory. Since -k is not passed in,
# the vendor/ directory from the tarball is explicitly deleted.
%goprep
# Unpack Source1 into the build tree, providing the vendor/ directory.
%setup -q -T -D -a1 -n %{name}-%{version}
# Apply patches, if present.
%autopatch -p1

%generate_buildrequires
%if 0%{?fedora}
# Generate data for the licence check go-vendor-tools provides.
%go_vendor_license_buildrequires -c %{S:2}
%endif

%build
export GO_LDFLAGS="-X github.com/redhatinsights/rhc/pkg/version.Version=%{version}"
%gobuild -o %{gobuilddir}/bin/rhc           %{goipath}/cmd/rhc
%gobuild -o %{gobuilddir}/bin/rhc-server    %{goipath}/cmd/rhc-server
%gobuild -o %{gobuilddir}/bin/rhc-collector %{goipath}/cmd/rhc-collector
%gobuild -o %{gobuilddir}/bin/com.redhat.minimal %{goipath}/cmd/minimal-collector

# Generate man page
%{gobuilddir}/bin/rhc --generate-man-page > rhc.1

%install
%if 0%{?fedora}
# Only go-vendor-tools are capable of collecting and packaging licenses for our dependencies.
%go_vendor_license_install -c %{S:2}
%endif

# Binaries
install -m 0755 -vd                     %{buildroot}%{_bindir}
install -m 0755 -vp _build/bin/rhc      %{buildroot}%{_bindir}/
install -m 0755 -vd                     %{buildroot}%{_libexecdir}/%{name}
install -m 0755 -vp _build/bin/rhc-server %{buildroot}%{_libexecdir}/%{name}/
install -m 0755 -vp _build/bin/rhc-collector %{buildroot}%{_libexecdir}/%{name}/
# Bash completion
install -m 0755 -vd                     %{buildroot}%{bash_completions_dir}/
install -m 0644 -vp data/completion/rhc.bash  %{buildroot}%{bash_completions_dir}/%{name}
# Logs
install -m 0755 -vd                     %{buildroot}%{_localstatedir}/log/%{name}/
# Collector directories
install -m 0755 -vd                     %{buildroot}%{_prefix}/lib/%{name}/collectors/
install -m 0755 -vd                     %{buildroot}%{_libexecdir}/%{name}/collectors/
# Logrotate
install -m 0755 -vd                     %{buildroot}%{_sysconfdir}/logrotate.d/
install -m 0644 -vp data/logrotate.d/rhc %{buildroot}%{_sysconfdir}/logrotate.d/rhc
# Man page
install -m 0755 -vd                     %{buildroot}%{_mandir}/man1
install -m 0644 -vp rhc.1               %{buildroot}%{_mandir}/man1/rhc.1
# Systemd files
install -m 0755 -vd                     %{buildroot}%{_unitdir}
install -m 0644 -vp data/systemd/rhc-canonical-facts.*  %{buildroot}%{_unitdir}/
install -m 0644 -vp data/systemd/rhc-server.service  %{buildroot}%{_unitdir}/
install -m 0644 -vp data/systemd/rhc-server.socket   %{buildroot}%{_unitdir}/
install -m 0644 -vp data/systemd/rhc-collector-com.redhat.minimal.*  %{buildroot}%{_unitdir}/
install -m 0755 -vd %{buildroot}%{_prefix}/lib/systemd/system-preset/
install -m 0644 -vp data/systemd/presets/50-rhc.preset %{buildroot}%{_prefix}/lib/systemd/system-preset/
# Configuration
install -m 0755 -vd                     %{buildroot}%{_sysconfdir}/%{name}/
# Minimal collector
install -m 0755 -vp _build/bin/com.redhat.minimal %{buildroot}%{_libexecdir}/%{name}/collectors/com.redhat.minimal
install -m 0644 -vp data/collectors/com.redhat.minimal.toml %{buildroot}%{_prefix}/lib/%{name}/collectors/

%if 0%{?with_rhcd_compat}
# Yggdrasil used to be called rhcd, and was part of rhc. For historical reasons, rhc
# still carries the compatibility shim to perform the rhcd->yggdrasil translation.
install -m 0755 -vd %{buildroot}%{_unitdir}/yggdrasil.service.d/
install -m 0644 -vp data/systemd/rhcd.conf %{buildroot}%{_unitdir}/yggdrasil.service.d/
%endif

%check
%if 0%{?fedora}
# Only go-vendor-tools are capable of validating the generated license string matches rpm's License field.
%go_vendor_license_check -c %{S:2}
%endif

%if %{with check}
# Trigger unit tests, unless explicitly disabled.
%gocheck
%endif

%if 0%{?with_rhcd_compat}
%pre
# On upgrade, back up /etc/rhc/config.toml before new files are laid down.
# This must happen in %pre (before file installation) rather than %post
# (after), because RPM may remove or replace the old config file during the
# transaction — by the time %post runs it could already be gone.
# The guard on /etc/yggdrasil/config.toml ensures we never overwrite a config
# that was already migrated by a previous upgrade.
if [ $1 -eq 2 ] && [ -f /etc/rhc/config.toml ] && [ ! -f /etc/yggdrasil/config.toml ]; then
	cp /etc/rhc/config.toml /etc/yggdrasil/config.toml.migrated
fi
%endif

%post
%systemd_post rhc-canonical-facts.timer
%systemd_post rhc-server.socket
%systemd_post rhc-collector-com.redhat.minimal.timer

%if 0%{?with_rhcd_compat}
# rhcd_t is the SELinux type used by the old rhcd daemon. Add it to the
# permissive list so existing policies do not block yggdrasil on upgrade.
/usr/sbin/semanage permissive --add rhcd_t || true

# Complete the config migration started in %pre: transform the backed-up
# /etc/rhc/config.toml into a valid /etc/yggdrasil/config.toml.
if [ $1 -eq 2 ] && [ -f /etc/yggdrasil/config.toml.migrated ]; then
	sed -E 's#^broker( ?=)#server\1#' /etc/yggdrasil/config.toml.migrated > /etc/yggdrasil/config.toml
	echo 'facts-file = "/var/lib/yggdrasil/canonical-facts.json"' >> /etc/yggdrasil/config.toml
	rm /etc/yggdrasil/config.toml.migrated
fi
%endif

%preun
%systemd_preun rhc-canonical-facts.timer
%systemd_preun rhc-server.socket rhc-server.service
%systemd_preun rhc-collector-com.redhat.minimal.timer

%postun
%systemd_postun_with_restart rhc-canonical-facts.timer
%systemd_postun_with_restart rhc-server.service
%systemd_postun_with_restart rhc-collector-com.redhat.minimal.timer

%if 0%{?with_rhcd_compat}
# Remove rhcd_t from the SELinux permissive list on full package removal.
if [ $1 -eq 0 ]; then
    /usr/sbin/semanage permissive --delete rhcd_t || true
fi
%endif

%if 0%{?fedora}
# With go-vendor-tools, we also get a list of dependency licenses.
%global extra_files -f %{go_vendor_license_filelist}
%else
%global extra_files %{nil}
%endif

%files %{extra_files}
# Binaries
%{_bindir}/rhc
%{_libexecdir}/%{name}/rhc-server
%{_libexecdir}/%{name}/rhc-collector
# Bash completion
%{bash_completions_dir}/%{name}
# Man page
%{_mandir}/man1/*
# Systemd
%{_unitdir}/rhc-canonical-facts.*
%{_unitdir}/rhc-server.service
%{_unitdir}/rhc-server.socket
%{_unitdir}/rhc-collector-com.redhat.minimal.*
%{_prefix}/lib/systemd/system-preset/50-rhc.preset
# Configuration
%{_sysconfdir}/%{name}/
# Collector directories
%dir %{_prefix}/lib/%{name}/collectors/
%dir %{_libexecdir}/%{name}/collectors/
# Minimal collector files
%{_libexecdir}/%{name}/collectors/com.redhat.minimal
%{_prefix}/lib/%{name}/collectors/com.redhat.minimal.toml
# Logs
%dir %{_localstatedir}/log/%{name}/
# Logrotate
%config(noreplace) %{_sysconfdir}/logrotate.d/rhc

%if 0%{?with_rhcd_compat}
# Yggdrasil rhcd compatibility drop-in
%{_unitdir}/yggdrasil.service.d/rhcd.conf
%endif

%changelog
%autochangelog
