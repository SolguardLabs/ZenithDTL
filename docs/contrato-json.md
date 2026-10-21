# Contrato de reporte JSON

`run <escenario> --json` emite una fotografía de la época. Los importes son enteros, los ratios se expresan en bps y el orden de filas es estable para facilitar comparaciones entre revisiones.

| Campo | Significado |
| --- | --- |
| `protocol`, `network`, `scenario`, `epoch` | Identidad de la ejecución. |
| `accounts`, `vaults` | Saldos y reservas internas. |
| `operators`, `channels`, `routes` | Reputación, límites y conectividad. |
| `capacity` | Plan, recomendación, asignación, exposición y headroom por operador. |
| `obligations`, `withdrawals` | Estados de settlement y retiradas. |
| `plans`, `reputation` | Historial de planes y scorecards. |
| `summary` | Conservación, totales y contadores de eventos. |
| `events` | Solo cuando se solicita la opción de eventos. |

`capacity.pendingAdjustments` expresa el neto de ajustes positivos y negativos aún diferidos, no el score futuro garantizado. `channelHeadroom` suma disponibilidad de canales; `assignedHeadroom` compara exposición abierta con el límite agregado. Ninguno es una cifra de fondos en custodia.

Ejemplo:

```bash
out/zenithdtl run balanced-reputation --json
out/zenithdtl validate balanced-reputation
```

Los consumidores deben preservar el ID de operador, ruta, canal y época junto a cada agregado; comparar cifras de épocas distintas sin esa clave puede producir conclusiones erróneas.
