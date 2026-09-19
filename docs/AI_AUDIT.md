# Tournoi reproductible des IA

`cmd/ai-audit` simule les niveaux originaux sans fenêtre ni audio. Il utilise les
ressources embarquées : données `level.dat`, économie des terrains `land0` à
`land3`, règles, populations initiales, mana, pouvoirs et cadence normaux. Aucun
bonus n'est accordé à l'IA stratégique. Le côté opposé utilise l'IA historique.

```sh
go run ./cmd/ai-audit -output /tmp/ai-campaign.csv
go run ./cmd/ai-audit -worlds 0,5,25,80,200,400 -side both -output /tmp/ai-panel.csv
```

Par défaut, le tournoi couvre les 495 mondes, avec l'IA stratégique côté bleu
(`-side 0`), quatre simulations parallèles au maximum (`-workers 4`) et une
limite de vingt minutes de jeu (`-duration 20m`), soit 9600 ticks à 8 Hz. La
simulation peut s'exécuter beaucoup plus vite que le temps du jeu. Les campagnes
sont asymétriques : inverser les côtés change aussi les ressources et pouvoirs
disponibles à chaque IA ; cela ne constitue pas un duel à ressources égales.

`-worlds` accepte `all`, une liste de numéros et des intervalles, par exemple
`0-19,200,450-494`. `-side both` joue chaque monde une fois de chaque côté. Le
fichier de sortie doit être nouveau ; une sortie existante n'est jamais écrasée.
Les résultats sont écrits dans l'ordre monde/côté, indépendamment du nombre de
workers. Ctrl-C conserve les matchs complètement simulés et exclut les matchs
interrompus ; le programme termine alors avec un statut d'échec.

## Résultats et limites des mesures

- `win` ou `loss` exige l'élimination de tous les groupes vivants d'un camp, en
  excluant les ruines. Le résultat est du point de vue de `strategic_side`.
- `draw` indique l'élimination simultanée. `ongoing` signifie que les deux camps
  vivent encore à la limite. Une avance de population ne vaut jamais victoire.
- `ticks` et `seconds` mesurent le temps simulé, pas le temps de calcul.
- `population` emploie le compteur standard du jeu ; `living_population` exclut
  explicitement les ruines. Les champs `initial_*` permettent de vérifier les
  ressources de départ.
- `castles`, `towns` et `peak_castles` échantillonnent les statistiques moteur
  rafraîchies au début du tick. Ils ne constituent pas un nouveau recalcul de la
  carte après la dernière action. Ces colonnes historiques sont conservées pour
  pouvoir relire les anciens audits.
- `actual_towns`, `actual_castles`, `actual_knights` et `actual_battles_won`
  sont au contraire recalculés depuis les personnages après le dernier tick.
  Les colonnes `peak_actual_*` donnent leur maximum observé après chaque tick ;
  `peak_actual_population` emploie le même total que `population` et inclut donc
  les ruines encore vivantes.
- `action_markers` compte uniquement les marqueurs `Computer.DoneTurn` encore
  observables à la fin du tick. Ils peuvent être réinitialisés à la frontière
  d'un créneau, notamment à vitesse 1 : ce sont des **bornes inférieures**, pas
  des nombres exacts d'actions ni un moyen de comparer les cadences des IA.
- Les compteurs `both_*`, `first_offense_tick` et `last_offense_tick` agrègent
  les événements sonores des **deux camps**. Les sons de marécage sont exclus,
  car ils peuvent venir de son déclenchement plutôt que de sa création. Ils
  restent présents pour la compatibilité des CSV existants.
- Les compteurs `blue_exact_*` et `red_exact_*` attribuent chaque sort réussi au
  bon camp à partir du score canonique ajouté pendant le tick. Ils couvrent
  séisme, marécage, chevalier, volcan, inondation et Armageddon, y compris le
  marécage qui ne possède pas d'événement sonore de lancement distinct. Une
  valeur non nulle dans `*_unclassified_power_score` signale une variation de
  score inattendue au lieu de l'attribuer arbitrairement à un sort.
- `slots` est la longueur allouée de la liste de groupes ; `alive_slots`,
  `dead_slots` et `ruins` en donnent la répartition.
- `state_hash` est l'empreinte déterministe finale du moteur. Comparer cette
  empreinte n'a de sens qu'à version identique du format d'état.

Les nouvelles colonnes exactes sont ajoutées après `state_hash`. Le préfixe du
format CSV, jusqu'à cette empreinte incluse, reste inchangé afin de préserver les
outils qui lisent les anciens audits par position.

Pour comparer une nouvelle stratégie, utiliser exactement le même moteur corrigé
des deux côtés de l'expérience, les mêmes mondes, côtés et limites. Conserver le
binaire témoin avant modification de la stratégie permet de rejouer un panel
sans remettre des fichiers anciens dans le répertoire de travail. Les résultats
de moteurs différents doivent rester des séries distinctes. Les mondes dont
l'issue régresse doivent être examinés individuellement, même lorsque le total
de victoires augmente.

Les tests du CLI vérifient notamment les sélecteurs, l'arrêt sur élimination
réelle, les ressources de départ, l'indépendance au nombre de workers et la
préservation des sorties existantes.

Le [bilan courant de l'IA stratégique](AI_STRATEGIC_REWORK_2026-09-19.md)
compare le même moteur avant/après les modifications du contrôleur. Les
[990 résultats témoins](AI_STRATEGIC_BASELINE_2026-09-19.csv) et les
[990 résultats courants](AI_STRATEGIC_CURRENT_2026-09-19.csv) sont conservés
avec leurs empreintes dans ce bilan. Les matchs encore ouverts à vingt minutes
restent exclus du nombre de victoires.
