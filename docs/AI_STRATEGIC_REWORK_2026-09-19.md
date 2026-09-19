# Refonte de l'IA stratégique à partir de l'IA originale

Date : 19 septembre 2026. Cette passe compare le C++ de
`previous/DCPopulous-master` au contrôleur stratégique Go de la démo. Elle ne
modifie ni l'IA historique utilisée dans les parties normales, ni les règles,
les ressources, la cadence ou le générateur aléatoire.

Dernière itération : **556 victoires sur 990 matchs (56,2 %)**, contre 363
avant cette passe. Les quatre terrains progressent, mais la neige reste sous
la majorité absolue : **82 victoires sur 210 (39,0 %)**. L'objectif de dominer
chaque terrain en moins de vingt minutes n'est donc pas encore atteint.

## Constat sur le plateau d'altitude 2

L'IA C++ ne choisit jamais l'altitude 2. `make_level` lit l'altitude courante du
centre de la ville, puis égalise autour de cette hauteur
(`populous_computer.cpp:21-79`). Le contrôleur Go ajoutait deux projets qui
réagissaient à la seule présence du pouvoir Flood chez l'adversaire :

- une rénovation de 64 sommets, soit au minimum 896 mana et 64 créneaux ;
- une extension pouvant atteindre 131 sommets autour de chaque zone, soit au
  minimum 1 834 mana, avant même les propagations du relief.

Le coût réel d'une action est `10 + 4 × sommets modifiés`. Les deux projets ont
été supprimés. Une ville est désormais améliorée à son altitude naturelle,
qu'elle soit 1, 2, 3 ou 4. Les anciens marqueurs de chantier éventuellement
présents dans un snapshot sont annulés proprement au lieu de bloquer l'IA.

## Optimisations retenues

- Les 17 cases nourricières utiles restent prioritaires par rapport au grand
  carré 9×9 de l'original.
- Une extension ordinaire dont la propagation dépasserait 84 mana est rejetée ;
  une action coûteuse reste possible si elle améliore immédiatement la ville.
- Une empreinte contenant une roche dure est abandonnée quand les colons peuvent
  légalement construire ailleurs. Dans les mondes « construction près des
  villes », les autres cases restent améliorées : cette ville peut être le seul
  point d'appui légal.
- Les reliefs de fondation de somme paire, ignorés par `one_block_flat` dans le
  C++, sont corrigés en choisissant le coin légal qui réduit le plus la pente au
  coût propagé le plus faible.
- Les nageurs récupérables et les colons réellement enfermés par l'eau ou un
  marais peuvent recevoir une action de terrain urgente. L'action est payée,
  exige une présence locale et ne doit ni noyer un allié ni dégrader une ville.
- Un chevalier n'est plus lancé avec seulement 1 000 ou 1 500 habitants. Le
  seuil historique supérieur à 3 000 évite de perdre 7 500 mana pour un raid
  trop fragile.
- Volcano, quatre fois plus cher que Quake, est réservé aux groupes adverses
  dont la valeur dépasse 12 000 ; une ville isolée reçoit plutôt un séisme si ce
  pouvoir est disponible.
- Sur désert et neige, un colon fragile sur une pente reçoit son action de
  fondation avant les travaux ordinaires. Le seuil est dérivé de `WalkDeath`.
- Le porteur du fanion reste dans sa ville pour atteindre la force d'un
  chevalier sur les paysages à forte attrition, au lieu d'attirer prématurément
  des recrues qui meurent en route.
- En dehors du recrutement local décrit ci-dessous, une nouvelle vague de
  chevalier n'est recrutée que si aucun chevalier allié n'est actif et qu'un
  groupe mobile dépasse déjà le seuil requis. Pour ce recrutement de réserve,
  le seuil est doublé dans l'économie neigeuse.
- Armageddon et Flood déjà disponibles passent avant le terrassement de front ;
  le chevalier fait de même uniquement sur les économies où l'A/B complet a
  progressé. Un chevalier réellement immobile peut uniquement revenir sur ses
  pas par un chemin sûr ; son déplacement normal reste inchangé.
- Une défense marécageuse ciblée peut couper la prochaine case d'un chevalier
  ennemi dirigé vers une cible amie, uniquement sur les économies où cette
  dépense a amélioré l'A/B et sans zone amie dans l'empreinte du sort.
- La prévision existante de Flood, l'évaluation des dégâts amis, les passages au
  front et l'épargne pour Armageddon sont conservés.

## Dernière passe : expansion légale et conversion de l'avance

### Débloquer les colons sans reprendre le privilège de l'original

Le C++ autorise son ordinateur à faire émigrer un château dès que sa population
dépasse 305, tout en gardant la production du château. Un joueur normal doit
attendre le dépassement de sa capacité de 3 050. Cette différence bloque
particulièrement l'expansion neigeuse ; elle n'est pas copiée dans la nouvelle
IA.

L'IA peut maintenant abaisser un sommet périphérique du terrain nourricier,
attendre le prochain véritable tick de croissance, puis remonter ce sommet.
La capacité temporairement réduite déclenche l'émigration ordinaire. Chaque
édition consomme sa mana et son créneau : 28 mana au total dans le cas simple,
56 au maximum dans la prévision acceptée. Aucun personnage ni point de
population n'est créé directement par le contrôleur.

Le départ est conditionné à une population suffisante pour l'attrition locale,
à de la place réellement colonisable à proximité, au nombre de marcheurs et
aux emplacements libres. Les voisins ne doivent perdre ni nourriture ni
habitants. Les règles « aucun terrassement » et « seulement élever » empêchent
ce projet. Sa restauration attend le tick de croissance même si un nouveau
créneau d'action s'ouvre plus tôt, et exige encore une présence de construction.
Son état est conservé dans les champs existants du snapshot.

Une autre correction porte sur la sélection des chantiers : les empreintes
impossibles à cause d'une roche dure sont exclues **avant** le classement des
quatre villes prioritaires. Elles ne peuvent plus occuper toute la liste et
empêcher le cinquième site, pourtant constructible, d'être amélioré.

### Conduire une expédition et recruter sans immobiliser les troupes

Quand ni chevalier ni Armageddon ne sont disponibles, une armée suffisamment
forte peut désormais suivre des étapes de fanion sur un chemin praticable.
La recherche évite eau, roches infranchissables et marais, tient compte de
l'attrition et de l'armement adverse, puis choisit un objectif atteignable par
le déplacement ordinaire. Chaque déplacement de fanion coûte 200 mana. Le
contrôleur ne déplace pas les personnages lui-même ; il réveille le porteur
avec une nouvelle commande payée lorsqu'une fusion ou une conquête l'a remis
en ville. L'expédition possède une échéance et abandonne si sa force s'effondre.

Sur l'économie aux productions intermédiaires identiques (neige), le fanion
peut rejoindre une ville amie assez peuplée pour recruter par fusion normale.
Ce trajet court remplace une attente statique très meurtrière : avec
`WalkDeath = 8`, attendre coûte huit habitants par tick, marcher huit par
cycle d'animation. La troupe regroupée devient un chevalier si ce pouvoir est
autorisé et payé, ou rejoint une expédition conventionnelle. Plusieurs vagues
sont possibles, avec une limite de trois chevaliers actifs pour ce recrutement.

### Adapter le risque au terrain

Sur les terrains à forte attrition, une défense par abaissement du sol peut
sacrifier une partie des fermes périphériques pour arrêter un chevalier. Avec
plus de 10 000 habitants et une supériorité supérieure à trois contre un, cette
même tactique peut attaquer une autre position ennemie. Les centres des villes
amies et les cases sèches occupées par les alliés restent protégés ; la présence
locale et la totalité du coût propagé restent obligatoires.

Flood peut aussi être retenu lorsque plus des deux tiers de l'ennemi seraient
exposés, que plus de la moitié de notre force resterait à l'abri et que les
survivants seraient encore plus de deux fois supérieurs. Le critère de pertes
absolues précédent reste disponible. Cette alternative est limitée aux terrains
à forte attrition après comparaison exhaustive.

Les économies à faible attrition conservent une préparation plus longue des
expéditions et la protection stricte des fermes. Les décisions dépendent des
règles de production et d'attrition, pas d'une liste de numéros de mondes.

Toutes les évaluations de terrain travaillent sur une copie déterministe. Elles
ne lisent pas les prochains tirages aléatoires. Les anomalies de l'IA originale
(roche effacée directement, terrassement sans mana, émigration privilégiée des
châteaux) ne sont pas reprises.

## Variantes mesurées puis rejetées

Le panel de 44 matchs a aussi servi à éliminer des idées intuitives mais moins
robustes : réserver systématiquement la mana d'un futur sort, donner tous les
pouvoirs avant le terrain offensif, imposer un quota de villages avant chaque
château, réactiver immédiatement le fanion après un chevalier et facturer le
coût complet dans tous les scores économiques. Sont aussi rejetés : le mode
Combat global déclenché par l'heure, le fanion global de nettoyage, un BFS qui
remplace tous les déplacements de chevaliers et un planificateur de château à
huit actions. Ce dernier passait ses tests mais faisait tomber le panel à 15
victoires et multipliait son temps d'audit par plus de trente. Ces variantes ne
sont pas livrées.

La dernière passe a également rejeté l'émigration trop précoce ou trop fréquente,
l'épargne militaire systématique, la réservation persistante d'Armageddon,
l'attente statique au fanion, l'attaque systématique de toutes les villes par
terrassement, ainsi que la généralisation aux économies peu meurtrières des
tactiques de pression retenues sur neige et désert. Un bonus persistant de
cible et une réserve supplémentaire contre Flood n'apportaient pas de victoire
supplémentaire ; ils ont aussi été retirés. Le choix final privilégie les
éliminations mesurées, pas le nombre de sorts lancés ni la seule population.

## Audit exhaustif actuel

`cmd/ai-audit` a simulé les 495 mondes pendant au plus 9 600 ticks (vingt
minutes), dans les deux camps. Une victoire exige l'élimination réelle. Les
ressources, pouvoirs et vitesses asymétriques de chaque niveau sont préservés :
ce n'est pas un tournoi à ressources égales. Le témoin ci-dessous est le
contrôleur au début de cette dernière passe, sur le **même moteur** : 363
victoires, 288 défaites et 339 limites. Les premières refontes avaient déjà fait
progresser le total de 268 à 363 ; elles ne sont pas comptées dans le gain actuel.

| Camp stratégique | Version | Victoires | Défaites | Nuls | Limite |
| --- | --- | ---: | ---: | ---: | ---: |
| Bleu, 495 mondes | Avant | 175 | 156 | 0 | 164 |
| Bleu, 495 mondes | Après | **269** | **86** | 0 | **140** |
| Rouge, 495 mondes | Avant | 188 | 132 | 0 | 175 |
| Rouge, 495 mondes | Après | **287** | **57** | 0 | **151** |
| Total, 990 matchs | Avant | 363 | 288 | 0 | 339 |
| Total, 990 matchs | Après | **556** | **143** | 0 | **291** |

Le gain net est de **193 victoires et 145 défaites de moins**. Les transitions
ne sont pas monotones : 98 anciennes défaites et 142 limites deviennent des
victoires ; 316 anciennes victoires sont conservées, 11 deviennent des défaites
et 36 atteignent la limite. La majorité absolue est atteinte globalement et dans
chaque camp, pas sur chacun des quatre terrains.

Résultats après refonte, deux camps réunis :

| Terrain | Victoires avant → après | Défaites avant → après | Limite après | Taux de victoire après |
| --- | ---: | ---: | ---: | ---: |
| Herbe, 270 matchs | 103 → **164** | 85 → **36** | 70 | 60,7 % |
| Désert, 290 matchs | 122 → **179** | 61 → **15** | 96 | 61,7 % |
| Neige, 210 matchs | 20 → **82** | 87 → **42** | 86 | 39,0 % |
| Rocheux, 220 matchs | 118 → **131** | 55 → **50** | 39 | 59,5 % |

Les mondes neigeux restent le principal chantier stratégique : les victoires
sont quatre fois plus nombreuses qu'avant, mais les parties encore ouvertes
restent nombreuses. Il faut mieux convertir l'avance économique en élimination,
notamment contre les positions isolées et lorsque les pouvoirs disponibles ne
permettent pas Armageddon. Ces résultats concernent la campagne testée ; ils ne
garantissent ni tous les mondes personnalisés, ni une victoire en vingt minutes.

## Reproduction et empreintes

```sh
go run ./cmd/ai-audit -worlds all -side both -workers 12 \
  -duration 20m -output /tmp/populous-ai-reworked-all.csv
```

L'audit ajoute maintenant, après l'ancien préfixe CSV, les sorts exacts par
camp et les pics réels de population, villes, châteaux, chevaliers et combats.
Les deux CSV détaillés sont conservés et inclus dans les exceptions de
`.gitignore` pour être ajoutables au prochain commit :

- [Témoin avant cette passe](AI_STRATEGIC_BASELINE_2026-09-19.csv).
- [Résultats actuels](AI_STRATEGIC_CURRENT_2026-09-19.csv).

Empreinte SHA-256 du témoin :

```text
dcc6beddc67026dba6f54712112c6594048ee306090e04bc081fb7c52452d3b5
```

Empreinte SHA-256 du résultat actuel :

```text
dc94b7ea7a30139bfbf84b6a83ce519ad4a7a4c17fab58e15f67146fbfe2530a
```

Les tests couvrent l'absence de cible altitude 2, l'annulation des anciens
chantiers, les roches dures, les pentes de fondation, les secours payants, le
seuil des chevaliers, le choix Quake/Volcano, les restrictions de construction
et le déterminisme après snapshot. Les nouveaux tests vérifient également le
tick réel d'émigration, les deux coûts de restauration, la sélection d'un site
derrière quatre chantiers impossibles, les recrutements et trajets au fanion,
la pression de terrain et les pertes projetées de Flood.

Validation de cette passe : suite complète avec et sans mode court, détecteur de courses
sur le moteur et l'audit, `go vet`, et oracle C++ optionnel. Ce dernier couvre
les terrains générés, certains combats et renforts, pas l'équivalence d'une
partie complète. Les références de l'IA historique restent inchangées. Les
microbenchmarks existants du contrôleur n'allouent pas de mémoire par itération.
À la suite de cette passe, les distributions ont été régénérées le 20 septembre
2026 : APK Android `1.1.0 (4)` et EXE Windows `1.1.0` pour x86, x64 et ARM64.
Le Pixel 10a a été mis à jour sans désinstallation avec la signature de
développement existante ; sa bibliothèque Go native est identique à celle de
l'APK public. Les sauvegardes et préférences sont conservées. Les contrôles de
construction et d'intégrité sont décrits dans les guides
[Android](ANDROID_RELEASE.md) et [Windows](WINDOWS_RELEASE.md).
