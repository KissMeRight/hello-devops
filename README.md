# hello-devops — Single-Node Kubernetes Web Stack

Go app → Docker (distroless, non-root) → k3s บน Ubuntu 24.04 (hardened)
พร้อม zero-downtime deployment และ graceful shutdown

## Stack
Ubuntu 24.04 LTS · Docker · k3s (Kubernetes 1.36) · Traefik · Go 1.24

## Architecture
    Client → Traefik (:80) → Service hello-svc → Pod x2
                                                  ├ ConfigMap (APP_ENV, GREETING, LOG_LEVEL)
                                                  ├ Secret    (API_KEY)
                                                  └ probes: startup → readiness → liveness

## Quick Start
    make build && make import
    cp k8s/20-secret.example.yaml k8s/20-secret.yaml   # แล้วแก้ค่า
    make deploy && make status

## Decision Log
| ตัดสินใจ | เหตุผล |
|---|---|
| k3s แทน kubeadm | binary เดียว RAM น้อย มี Traefik/CoreDNS/local-path มาให้ API เหมือน k8s |
| Traefik แทน ingress-nginx | ingress-nginx ถูกปลดระวางมี.ค. 2026 ไม่มี security patch |
| Kubernetes 1.36 แทน 1.37 | 1.37 เพิ่งออก ส.ค. 2026 รอให้ ecosystem ตามทัน |
| distroless + non-root | ไม่มี shell ลด attack surface, image ~8MB |
| maxUnavailable: 0 | capacity เต็มตลอดช่วง rolling update |
| PSA restricted | บังคับ non-root/drop capabilities ระดับ namespace |
| import image ตรงเข้า containerd | ไม่พึ่ง registry ภายนอกในแล็บ |

## Verified Scenarios
- ลบ pod ระหว่างยิง traffic จากเครื่องอื่น → ไม่มี HTTP error
- Rolling update 0.1.0 → 0.2.0 ไม่มี downtime, rollback < 15 วินาที
- Debug ImagePullBackOff / OOMKilled / endpoints ว่าง / CrashLoopBackOff (ดู docs/troubleshooting.md)

## Server Hardening
SSH key only, root login ปิด, ufw default deny, fail2ban, swap ปิด, healthcheck cron ทุก 5 นาที
