# Recherche de stratégies — 19 septembre 2026

Expériences isolées : aucun changement du code de production. Le harness est conservé dans `/private/tmp/populous-strategy-fixed/research_strategy_test.go`. Les overlays Go remplaçaient le point d'appel du contrôleur avancé, sans modifier l'ordre de simulation, les profils originaux, les ressources, les pouvoirs permis, les cadences d'action, le RNG ou le résultat de partie.

## Protocole

- IA nouvelle du côté God (bleu), IA historique côté Devil. Cela ne prouve pas la performance avec inversion des camps.
- `GenerateWorldWithRules(level, DecodeTerrainRules(landN))`, données `level.dat` authentiques.
- Arrêt à élimination réelle (hors ruines) ou 9600 ticks = 1200 secondes = 20 minutes. Les timeouts ne sont jamais comptés comme victoires, même avec une population largement supérieure.
- Panel principal de 9 mondes : 0, 5, 25, 80, 100, 200, 250, 400, 450.
- Validation séparée de 10 profils distincts : 10, 35, 70, 130, 160, 220, 300, 350, 475, 490. Aucune règle par numéro de monde ; aucun ajustement effectué après lecture des résultats de validation.
- Deux moteurs : code courant ; overlay expérimental réparant symétriquement le recyclage des morts et le passage des chevaliers à travers les villes amies, fourni par `engine_strategy_audit`. Les deux camps bénéficient des mêmes corrections.

## Contrôleurs étudiés

0. IA actuelle, témoin.
1. Économie/Armageddon : conserver Settle, aménager normalement, renoncer au harcèlement si Armageddon est autorisé pour épargner réellement 80 000 mana ; lancer à population > 1,5 fois celle de l'adversaire. Sans Armageddon, conserver sorts actuels et Settle.
2. Même chose, seuil 1,2. Contrôle de sensibilité uniquement sur les 9 mondes principaux.
3. Diffusion d'abord : durant les cinq premières minutes et tant que le réseau a moins de 10 villes, aplanir de petits voisinages (objectif 185 nourriture), éviter la promotion prématurée en château ; ensuite aménagement normal. Sorts actuels, Settle conservé.
4. Soutien offensif : terrassement légal au front pour déclasser/noyer des positions adverses, ou créer une traversée pour leader/chevalier ; réserve minimale, coût réel complet, présence locale 8×8 conforme, aucun village ami déplacé ni unité amie noyée. Politique d'expéditions avec chevalier à partir de 1000 personnes, regroupement ou déplacement du leader selon les forces ; éviter Fight permanent. Sorts actuels et économie normale en repli.
5. Suppression du délai de grâce offensif contre les profils passifs ; le reste reste identique. Cela ne suffit pas à produire une victoire supplémentaire.
6. Émigration pulsée : déclasser temporairement un coin périphérique d'un château peuplé, attendre au moins un cycle normal de naissance, puis restaurer le terrain. Deux actions et leurs vrais coûts ; pas de bonus artificiel de population. Exclut les règles interdisant de baisser, les implantations sans espace et les dommages aux autres villes. Testé sur le moteur réparé.

## Résultats comparables

| Contrôleur | Moteur courant, 9 mondes | Courant, 10 profils réservés | Réparé, 9 mondes | Réparé, 10 profils réservés |
|---|---:|---:|---:|---:|
| Actuel | 2 victoires | 0 | 4 | 5 |
| Banque 1,5 | 1 | 1 | 4 | non testé |
| Banque 1,2 | 2 | non testé | non testé | non testé |
| Diffusion villages | 2 | 2 | 4 | 5 |
| Soutien offensif | 2 | 2 | 5 | 6 |
| Déclenchement offensif précoce | 2 | 0 | non testé | non testé |
| Émigration pulsée | non testé | non testé | 2 | 4 |

Meilleur résultat de ce petit panel : soutien offensif sur moteur réparé, **11 victoires sur 19, 8 parties non terminées, aucune défaite**, contre **9 victoires, 9 parties non terminées et 1 défaite** pour l'IA actuelle sur le même moteur. Le panel n'est pas un échantillon aléatoire des 495 mondes : ne pas extrapoler ce pourcentage à toute la campagne.

Exemples du moteur réparé :

| Monde | IA actuelle | Soutien offensif | Autre piste utile |
|---|---|---|---|
| 5 | non terminé à 20 min | victoire 804,125 s | banque : 915,250 s |
| 10 | non terminé à 20 min | victoire 651,375 s | diffusion : défaite 881 s |
| 25 | non terminé à 20 min | non terminé | diffusion : victoire 600,250 s ; banque : 1035,625 s |
| 100 | victoire 629,250 s | victoire 665,125 s | — |
| 200 | victoire 971,375 s | victoire 817,875 s | diffusion : 430,375 s ; émigration pulsée : 337,250 s |
| 250 | victoire 480,375 s | victoire 1063 s | banque : défaite 803,625 s |
| 350 | victoire 797,250 s | victoire 473,625 s | émigration pulsée : 443,500 s |
| 475 | victoire 620,250 s | victoire 354,500 s | émigration pulsée : non terminé |

Les variantes économiques ne sont donc pas des améliorations générales. Le soutien offensif est la piste la plus robuste observée, mais ne satisfait pas encore l'objectif universel de 20 minutes. Les mondes sans construction 160 et 300 restent non terminés dans toutes ces expériences ; il faut une stratégie de déplacement/occupation exploitant le terrain préexistant, pas un autre algorithme d'aplanissement.

## Recommandations issues des essais

1. Corriger d'abord les divergences moteur symétriques confirmées par comparaison au C++ original ; elles changent davantage le résultat que les seuls seuils de sorts.
2. Choisir la stratégie selon les règles et l'état présent : rendement réel du terrain, pouvoirs disponibles des deux camps, possibilités d'expansion, chemins vers l'ennemi, forces effectivement mobilisables. Pas selon un numéro de monde mémorisé.
3. Garder une armée mobile et soutenir son passage avec les actions normales. Déclasser une ville ennemie de près peut coûter quelques dizaines de mana, contre des milliers pour un sort à effet aléatoire.
4. Mesurer le retour d'un sort en villes arrêtées, capacité productive supprimée et ennemis éliminés, plutôt que répéter des séismes dès qu'ils deviennent abordables.
5. Préserver les petits établissements assez longtemps pour diffuser ; promouvoir ensuite les sites les plus rentables. L'émigration pulsée est intéressante, surtout quand les châteaux ont une croissance élevée et peu d'avantage de mana, mais ne doit pas devenir une règle systématique.
6. Ne pas décider Armageddon sur le seul rapport de populations : les regroupements plafonnent à 32 000, les itinéraires et l'équipement comptent, et certaines finales restent bloquées même après son lancement.
7. Avant adoption, tournoi 495 mondes × 2 camps, puis variantes de graines. Exiger des éliminations avant 9600 ticks, surveiller chaque régression, et conserver les fins non terminées comme échecs de l'objectif.

## Artifacts

Résultats courants : `/private/tmp/populous-strategy-lab/results-v012.txt`, `results-v3.txt`, `results-v4.txt`, `results-v5.txt`, `results-heldout.txt`.

Résultats moteur réparé : `/private/tmp/populous-strategy-fixed/results.txt`, `results-heldout.txt`, `results-v6.txt`.

Overlays et harness conservés dans les mêmes répertoires. Pour reproduire, remettre temporairement `research_strategy_test.go` dans `internal/populous`, puis exécuter `POPULOUS_RESEARCH=1 RESEARCH_VARIANT=0,3,4 go test -overlay /private/tmp/populous-strategy-fixed/overlay.json ./internal/populous -run '^TestResearchCandidates$' -count=1 -v`. `RESEARCH_LEVELS` permet de fournir la liste des mondes séparés par des virgules. Supprimer ensuite uniquement ce fichier de recherche temporaire.
