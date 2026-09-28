# Troubleshooting Log

## 1. ImagePullBackOff / ErrImagePull
- อาการ: pod ค้าง 0/1 ImagePullBackOff
- ตรวจ: kubectl describe pod <name> → ดู Events
- สาเหตุ: tag ไม่มีอยู่จริง / image ไม่ได้ import เข้า containerd ของ k3s
- แก้: แก้ tag หรือ docker save | sudo k3s ctr images import -
- เรียนรู้: maxUnavailable: 0 ทำให้ pod เก่ายังให้บริการ ผู้ใช้ไม่กระทบ

## 2. OOMKilled (Exit Code 137)
- อาการ: pod restart วน, Last State: OOMKilled
- ตรวจ: kubectl describe pod | grep -A6 "Last State"
- สาเหตุ: memory limit ต่ำกว่าที่แอปใช้จริง
- เรียนรู้: memory เกิน limit = ถูกฆ่าทันที ต่างจาก CPU ที่แค่ถูก throttle

## 3. Service ไม่มี endpoint
- อาการ: เข้าเว็บได้ 503, kubectl get endpoints เป็น <none>
- สาเหตุ: selector ของ Service ไม่ตรง label ของ Pod หรือ readiness ไม่ผ่าน
- เรียนรู้: debug Service ให้ดู endpoints เป็นอันดับแรก

## 4. CrashLoopBackOff จาก liveness probe
- อาการ: RESTARTS เพิ่มเรื่อยๆ
- ตรวจ: kubectl logs <pod> --previous + kubectl describe pod
- สาเหตุ: path ของ livenessProbe ผิด → kubelet ฆ่า container ซ้ำ
- เรียนรู้: liveness ที่ตั้งผิดอันตรายกว่าไม่ตั้ง ใช้ startupProbe กับแอปที่ boot ช้า
