#
# spec file for package elemental-agent
#
# Copyright (c) 2023 SUSE LLC
#
# All modifications and additions to the file contributed by third parties
# remain the property of their copyright owners, unless otherwise agreed
# upon. The license for this file, and modifications and additions to the
# file, is the same license as for the pristine package itself (unless the
# license for the pristine package is not an Open Source License, in which
# case the license is the MIT License). An "Open Source License" is a
# license that conforms to the Open Source Definition (Version 1.9)
# published by the Open Source Initiative.

# Please submit bugfixes or comments via https://bugs.opensuse.org/
#

# These variables are coupled to automation scripts
%define commit _replaceme_
%define c_date _replaceme_

%define pluginsdir /usr/lib/elemental/plugins

Name:           elemental-agent
Version:        0
Release:        0
Summary:        Elemental CAPI agent
License:        Apache-2.0
Group:          System/Management
URL:            https://github.com/rancher-sandbox/cluster-api-provider-elemental
Source:         %{name}.tar.xz
Source1:        %{name}.rpmlintrc
Requires:       elemental-plugin

BuildRequires:  make

BuildRequires:  golang(API) >= 1.22
BuildRequires:  golang-packaging
%{go_provides}

BuildRoot:      %{_tmppath}/%{name}-%{version}-build

%description
The Elemental CAPI agent is responsible for managing the OS 
versions and maintaining a machine inventory to assist with edge or 
baremetal installations.

%package -n elemental-systemd-services
Summary: Elemental CAPI agent systemd services
Requires: elemental-agent = %{version}-%{release}
Requires: elemental-plugin-toolkit = %{version}-%{release}
%{?systemd_requires}
%description -n elemental-systemd-services
This package contains systemd services to run the elemental-agent 
when the elemental-plugin-toolkit is also in use.

%package -n elemental-plugin-toolkit
Summary: Provides the elemental plugin 
Provides: elemental-plugin
Requires: elemental-agent = %{version}-%{release}
Requires: elemental-toolkit
%description -n elemental-plugin-toolkit
The toolkit plugin allows integration between the elemental-toolkit 
and the elemental-agent.

%package -n elemental-plugin-dummy
Summary: Provides a dummy plugin
Provides: elemental-plugin
Requires: elemental-agent = %{version}-%{release}
%description -n elemental-plugin-dummy
The dummy plugin is a very basic plugin for the elemental-agent 
that can be used for debugging, or when no other plugin option 
is available.

%prep
%setup -q -n %{name}

%build
%goprep .

if [ "%{commit}" = "_replaceme_" ]; then
  echo "No commit hash provided"
  exit 1
fi

if [ "%{c_date}" = "_replaceme_" ]; then
  echo "No commit date provided"
  exit 1
fi

export GIT_TAG=$(echo "%{version}" | cut -d "+" -f 1)
GIT_COMMIT=$(echo "%{commit}")
export GIT_COMMIT=${GIT_COMMIT:0:8}
export GIT_COMMIT_DATE="%{c_date}"

mkdir -p bin
make build-agent
make build-plugins

%install
%goinstall

%{__install} -d -m 755 %{buildroot}%{_sbindir}
%{__install} -d -m 755 %{buildroot}%{pluginsdir}

%{__install} -m 755 bin/elemental-agent %{buildroot}%{_sbindir}
%{__install} -m 755 bin/elemental.so %{buildroot}%{pluginsdir}
%{__install} -m 755 bin/dummy.so %{buildroot}%{pluginsdir}

mkdir -p %{buildroot}%{_unitdir}
cp -a framework/files/usr/lib/systemd/system/* %{buildroot}%{_unitdir}
%pre -n elemental-systemd-services
%service_add_pre elemental-agent.service
%service_add_pre elemental-agent-install.service

%post -n elemental-systemd-services
%service_add_post elemental-agent.service
%service_add_post elemental-agent-install.service

%preun -n elemental-systemd-services
%service_del_preun elemental-agent.service
%service_del_preun elemental-agent-install.service

%postun -n elemental-systemd-services
%service_del_postun elemental-agent.service
%service_del_postun elemental-agent-install.service

%files
%defattr(-,root,root,-)
%license LICENSE
%{_sbindir}/%{name}
%dir /usr/lib/elemental
%dir %{pluginsdir}

%files -n elemental-systemd-services
%defattr(-,root,root,-)
%license LICENSE
%dir %{_unitdir}
%{_unitdir}/elemental-agent.service
%{_unitdir}/elemental-agent-install.service

%files -n elemental-plugin-toolkit
%defattr(-,root,root,-)
%license LICENSE
%dir %{pluginsdir}
%{pluginsdir}/elemental.so

%files -n elemental-plugin-dummy
%defattr(-,root,root,-)
%license LICENSE
%dir %{pluginsdir}
%{pluginsdir}/dummy.so

%changelog
