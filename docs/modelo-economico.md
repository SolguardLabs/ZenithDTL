# Modelo económico

Los importes son enteros en unidades internas de `ZUSD`. Los fees y porcentajes se expresan en puntos básicos (`10 000 bps = 100 %`). El motor no realiza conversión de divisas ni valoración externa.

## Capacidad por operador

La recomendación parte de `baseCapacity`, score reputacional, stake y peso de ruta. En forma simplificada:

```text
capacidad_bruta = (base + floor(base × score / 1 000) + floor(stake / 5))
                 × peso_ruta / 100
capacidad_planificada = min(max(capacidad_bruta, mínimo), máximo)
```

Después se aplican límites de estado (por ejemplo watchlist o suspensión) y se divide la capacidad planificada entre canales por pesos de modo/prioridad. El cálculo usa enteros; los restos de división se asignan al último canal para que las partes sumen el total.

Ejemplo ilustrativo: con base `100 000`, score `800`, stake `50 000` y peso `100`, la capacidad bruta es `100 000 + 80 000 + 10 000 = 190 000`, antes de los topes de política. Con dos canales de pesos 3:2, el reparto es `114 000` y `76 000`.

## Obligaciones y rutas

```text
fee_ruta = ceil(importe × fee_bps_total / 10 000)
bruto_reservado = importe + fee_ruta
necesidad_de_canal = importe + ceil(importe × fee_bps_del_salto / 10 000)
disponible_canal = max(límite − used − reserved − inFlight, 0)
exposición_abierta = suma de necesidades de canal en obligaciones abiertas
```

Al liquidar una obligación, el beneficiario recibe el importe y cada operador recibe su parte del fee; el remanente va a tesorería. Al cancelar o expirar, se liberan reservas. La métrica `capacity.utilizationBps` divide exposición abierta por límite asignado; `assignedHeadroom` no representa saldo de caja.

## Riesgo y límites

El reporte distingue `plannedLimit`, `recommendedLimit`, `assignedLimit`, exposición abierta y headroom de canales. Estos valores describen superficies distintas: reputación, plan, distribución y compromisos. No deben sumarse entre activos o tratarse como fondos liquidados. La conservación del ledger verifica asientos internos, no la solvencia de un operador fuera del motor.
