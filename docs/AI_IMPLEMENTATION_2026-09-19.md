# Intégration des corrections moteur et des tactiques

Cette implémentation fait suite à [l’étude des 495 mondes](AI_STRATEGIES_20_MIN.md).
Les corrections s’appliquent aux deux camps. Les nouvelles décisions tactiques
concernent uniquement l’IA stratégique, utilisée par la démo et l’audit.

## Corrections livrées

- Les villes réutilisent le premier emplacement de groupe mort. Les anciens
  champs et références carte/fanion sont nettoyés ; la division de population
  conserve les habitants. La limite de 208 groupes effectivement vivants reste
  inchangée.
- Un chevalier continue sa marche lorsqu’une ville amie refuse de le fusionner.
  Une fusion ordinaire consomme toujours le marcheur et un contact ennemi
  déclenche toujours le combat.
- Les tests couvrent les deux camps, les tableaux saturés, la réallocation,
  les références périmées et le déterminisme après snapshot.

Les autres divergences de déplacement identifiées par l’étude (attrition,
évitement des marais pendant Armageddon) ne sont pas modifiées par ce lot.

## Améliorations tactiques livrées

- **Soutien offensif local** : dégrader la nourriture d’une ville ennemie ou
  submerger une position lorsque la présence de ses propres troupes permet
  légalement le terrassement.
- **Passages pour les expéditions** : aménager une traversée sur l’eau devant
  un chef ou chevalier. Le déplacement et les combats restent ceux du moteur.
- **Rassemblements militaires** : concentrer les marcheurs autour du chef avant
  de le convertir en chevalier ; employer le fanion pour une expédition lorsque
  le rapport de forces le permet. La politique revient à la colonisation en
  dehors de ces phases, plutôt que rester constamment en mode combat.
- **Prévision de Flood** : appliquer la transformation déterministe du terrain
  sur une copie pour compter les populations réellement submergées, sans lire
  les prochains tirages aléatoires ni jouer un sort pendant l’évaluation.
- **Restrictions de construction** : vérifier la présence locale pour chaque
  essai et chaque modification, y compris à la reprise d’une réparation.
- **Mondes sans construction** : passer en mode Combat normal après quatre
  villes, sans attendre trois châteaux. Les villes continuent de produire ;
  aucune troupe n’est déplacée ou créée artificiellement. Sur les dix mondes
  concernés joués des deux côtés, le bilan passe de 1 victoire, 1 défaite et
  18 inachevés à 3 victoires, 1 défaite et 16 inachevés. Les seuils de deux,
  quatre et huit villes donnent les mêmes trois victoires dans cet essai.

Les coûts de mana, pouvoirs et créneaux d’action restent normaux. Aucun nouveau
champ de simulation caché n’est introduit ; les plans continuent d’utiliser les
champs déjà sauvegardés. La priorité générale aux petits villages, l’émigration
provoquée par déclassement des châteaux et les variantes de recrutement
automatique ne sont pas activées : les essais ont montré des régressions et ne
justifient pas leur généralisation.

## Validation comparative

La référence utilise **le moteur corrigé avec l’ancienne stratégie**. Comparer
uniquement au moteur défectueux surestimerait le progrès propre de l’IA.

Le premier tournoi exhaustif de référence, camp bleu, donne **175 victoires,
43 défaites et 277 parties inachevées**, contre 88 victoires, 45 défaites,
1 nul et 361 inachevées avant les corrections moteur. Chaque monde conserve
ses règles, ressources et pouvoirs. Une victoire exige une élimination réelle
avant ou au tick 9 600 (vingt minutes à 8 Hz).

Les inversions de camps ne rééquilibrent pas les ressources asymétriques des
niveaux. Elles servent à contrôler le fonctionnement du contrôleur des deux
côtés, et doivent être présentées séparément du résultat de la démo normale.

Résultats exhaustifs, même moteur corrigé avant et après les changements d’IA :

| Camp stratégique | Version | Victoires | Défaites | Nuls | Inachevés à 20 min |
| --- | --- | ---: | ---: | ---: | ---: |
| Bleu, 495 mondes | Ancienne IA | 175 | 43 | 0 | 277 |
| Bleu, 495 mondes | Nouvelle IA | **218** | **39** | 1 | **237** |
| Rouge, 495 mondes | Ancienne IA | 150 | 27 | 0 | 318 |
| Rouge, 495 mondes | Nouvelle IA | **234** | **26** | 0 | **235** |
| Total, 990 matchs | Ancienne IA | 325 | 70 | 0 | 595 |
| Total, 990 matchs | Nouvelle IA | **452** | **65** | 1 | **472** |

Les 990 clés monde/camp sont uniques et complètes. Règles et ressources
initiales correspondent entre les deux versions. Les résultats et la limite
exacte ont été contrôlés à partir des populations vivantes, hors ruines.

Le gain net est de **127 victoires**, mais il comprend **198 nouvelles victoires
et 71 anciennes victoires perdues**. Côté bleu : 82 gains et 39 régressions ;
côté rouge : 116 gains et 32 régressions. L’objectif universel n’est donc pas
atteint, et cette version n’est pas meilleure sur chaque carte.

Régressions prioritaires passant d’une victoire à une défaite : mondes bleus
**2, 155, 157, 269, 288, 368** ; mondes rouges **204, 269, 324, 366, 395**.
Les autres anciennes victoires perdues deviennent inachevées ou nulles.
Les mondes rocheux restent particulièrement fragiles. Les résultats complets,
les transitions et les catégories figurent dans le
[CSV comparatif](ai-results/comparison.csv) et le
[rapport détaillé](ai-results/comparison-report.txt).

Exemples côté bleu : monde 10, inachevé → victoire à **482,5 s** ; monde 19,
défaite → victoire à **894,875 s** ; monde 304 sans construction, inachevé →
victoire à **869,75 s**. Ces exemples ne remplacent pas le bilan exhaustif.

### Provenance des résultats finaux

Le tournoi a d’abord figé l’IA avant l’optimisation équivalente et avant le
raffinement `GameNoBuild`. L’optimisation est contrôlée par les 38 parties
strictement identiques décrites ci-dessous. Après le raffinement NoBuild, seuls
les 20 matchs concernés ont été rejoués, indépendamment par deux contrôleurs
de validation, avec des CSV identiques. Les 970 autres lignes sont conservées
sans changement : la nouvelle branche est strictement conditionnée par
`GameNoBuild`. Seize matchs hors NoBuild confirment également l’identité des
empreintes avant/après ce raffinement.

Empreintes SHA-256 de la version livrée :

```text
world.go            550951dbcc830c70ceababc760f7c3693feec2cc8bfd3a59692aae2bfd23c7d7
advanced_ai.go      e72e64d164dd6c4706df887f2aa8795314c643100e0ec11b93cc0055f40bb41d
advanced_tactics.go 92edd57608fa2f52c9b5c3b385f224120cbcd8310fea462af6a27190988cadb2
comparison.csv     08ddb010519067c635d34ce3511bafff3fe4875ee6c672115c590f3ed91846df
```

Les jeux complets avec davantage de compteurs restent disponibles dans
`/private/tmp/populous-audit-engine-fixed-baseline.csv`,
`/private/tmp/populous-audit-engine-fixed-baseline-side1.csv` et
`/private/tmp/populous-audit-final-merged-both.csv`. Le CSV compact dans le dépôt
conserve notamment les résultats, durées, populations initiales et empreintes
finales des deux versions, permettant de vérifier les comparaisons sans les
fichiers temporaires.

## Optimisation conservant les décisions

Le calcul du terrassement vérifie la présence avant la copie du monde,
déduplique les candidats identiques en conservant leur ordre, et met en cache
la nourriture initiale des villes adverses. Un évaluateur de référence conservé
dans les tests vérifie le choix des actions, notamment aux frontières, sur les
ponts et avec 208 groupes se recouvrant.

Sur 19 mondes joués des deux côtés, les **38 parties** produisent des CSV
strictement identiques avant et après optimisation, empreintes finales incluses.
Empreinte SHA-256 commune :
`eff75235fb395014562948c39ef4c6c559a49567fa5bff8a1e5f40c7f8bce2d6`.

Mesures indicatives sur Apple M4 Max : 0,341 → 0,294 ms par tick représentatif,
9,412 → 7,245 ms sur le scénario artificiel de 208 villes ; zéro allocation dans
les deux cas. Ces temps dépendent de la machine et de sa charge.

## Sauvegardes, réseau et reproduction

Le format des sauvegardes et des snapshots reste inchangé ; une ancienne
sauvegarde reprend avec les nouvelles règles. En revanche, les résultats de
simulation changent : le préfixe de compatibilité réseau est maintenant
`go-populous-lockstep-2`. Les anciens exécutables sont refusés avant la création
d’une session. La représentation des empreintes d’état reste en version 2.

Le nouvel outil [`cmd/ai-audit`](../cmd/ai-audit) fonctionne sans fenêtre ni audio.
Son [guide](AI_AUDIT.md) décrit les options et les limites des métriques.

```sh
go run ./cmd/ai-audit -output /tmp/populous-ai.csv
go run ./cmd/ai-audit -side both -output /tmp/populous-ai-both.csv
```

Les tests, le contrôle de concurrence et `go vet` couvrent le moteur, les
tactiques, le protocole et le nouvel outil. La compilation de l’application de
bureau est également vérifiée. Une démo complète du monde 10 a été exécutée
dans l’application : victoire bleue à 482,5 secondes, rendu et export MP4
avec audio interne contrôlés.

La version courante a aussi été compilée pour Android arm64 et installée sur
le Pixel 10a par `scripts/run-android.sh` : signature et alignement 16 KiB
vérifiés, mise à jour `install -r` réussie, processus démarré sans plantage
immédiat détecté. La sauvegarde existante est toujours présente. Le téléphone
n’a pas été déverrouillé pour forcer un essai interactif ; cette vérification
ne remplace donc pas le test tactile par l’utilisateur après déverrouillage.

Aucun commit ni push n’est effectué.
