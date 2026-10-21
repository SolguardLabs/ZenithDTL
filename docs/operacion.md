# Operación y recuperación

## Ciclo habitual

1. Registrar la revisión de código y ejecutar `npm run ci`.
2. Consultar `list` y ejecutar el escenario que corresponda al ciclo operativo.
3. Revisar rutas, scorecards, capacidad por canal, `capacity` por operador y obligaciones abiertas.
4. Conciliar balances de cuenta y bóveda, fees y retiros pendientes con el diario.
5. Ejecutar `validate` y conservar reporte y comando exacto antes de avanzar época.

```mermaid
flowchart LR
    I[Instrucción] --> R[Validación de ruta]
    R --> B[Reserva]
    B --> E[Cierre de época]
    E --> C[Capacity digest]
    C --> S[Settlement / cancelación]
    S --> A[Conciliación]
    A -->|diferencia| H[Retener nuevo ciclo]
```

Una variación de score, un canal con utilización elevada, una obligación cercana al vencimiento o una divergencia entre recomendación y capacidad asignada requieren revisión humana. El reporte ayuda a localizar el operador y la época; no reemplaza la conciliación con sistemas externos.

Ante una discrepancia, no reenvíe la misma obligación sin una clave de idempotencia de la integración. Guarde IDs de obligación/ruta/canal, época, balance anterior y posterior y eventos. Suspenda el siguiente ciclo hasta determinar si el estado terminal fue registrado. Este ejecutable local no revierte pagos externos.

```bash
out/zenithdtl run capacity-limits --json
out/zenithdtl run penalty-cycle --json
out/zenithdtl validate route-withdrawal
```
