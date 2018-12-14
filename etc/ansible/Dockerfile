FROM centos:centos7

RUN yum -y update; yum clean all
RUN yum -y install sudo; yum clean all
RUN yum -y install systemd; yum clean all; \
                                         (cd /lib/systemd/system/sysinit.target.wants/; for i in *; do [ $i == systemd-tmpfiles-setup.service ] || rm -f $i; done); \
                                         rm -f /lib/systemd/system/multi-user.target.wants/*;\
                                         rm -f /etc/systemd/system/*.wants/*;\
                                         rm -f /lib/systemd/system/local-fs.target.wants/*; \
                                         rm -f /lib/systemd/system/sockets.target.wants/*udev*; \
                                         rm -f /lib/systemd/system/sockets.target.wants/*initctl*; \
                                         rm -f /lib/systemd/system/basic.target.wants/*;\
                                         rm -f /lib/systemd/system/anaconda.target.wants/*;
VOLUME ["/sys/fs/cgroup"]
RUN yum -y install openssh-server passwd; yum clean all

# docker stops container without this line
RUN ssh-keygen -t rsa -f /etc/ssh/ssh_host_rsa_key -N ''
# TODO: simplify method for keep container in background

ADD ./sshd_config /etc/ssh/sshd_config
ADD ./id_rsa.pub /~/.ssh/authorized_keys

CMD ["/usr/sbin/init"]
