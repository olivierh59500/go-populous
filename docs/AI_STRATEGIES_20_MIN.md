# Étude de l’IA : gagner légalement en vingt minutes

> Archive de l’étude initiale, avant intégration des corrections moteur et des
> nouvelles tactiques. Les chiffres ci-dessous décrivent cette ancienne
> version ; le bilan de l’implémentation figure dans
> [AI_IMPLEMENTATION_2026-09-19.md](AI_IMPLEMENTATION_2026-09-19.md).

Date : 19 septembre 2026. Périmètre : les **495 mondes originaux livrés avec le projet**, IA stratégique bleue contre IA historique rouge, pouvoirs et ressources de chaque niveau conservés.

**Conclusion : l’objectif n’est pas atteint.** L’audit exhaustif donne **88 victoires en moins de vingt minutes sur 495 mondes**. Des stratégies prometteuses existent, mais plusieurs défauts du moteur faussent actuellement leur évaluation. Cette étude ne démontre ni une garantie de victoire universelle, ni l’impossibilité d’en obtenir une sur cette campagne fixe.

Cette passe conserve le code du jeu : les variantes ont été exécutées dans des copies de diagnostic avec `go test -overlay`. Aucun bonus de mana, de population, de cadence ou de pouvoirs n’a été accordé à l’IA. Les prototypes décrits ci-dessous ne sont pas activés dans le jeu.

## Mesure exhaustive de la version actuelle

Une partie est simulée pendant au plus **9 600 ticks à 8 Hz**, soit vingt minutes de jeu. Seule l’élimination de tous les personnages vivants non réduits en ruines constitue une victoire. Une avance de population ne suffit pas. Le nul correspond à une double élimination ; la limite signifie que les deux camps restent vivants.

| Terrain | Mondes | Victoires | Défaites | Nuls | Inachevés à 20 min |
| --- | ---: | ---: | ---: | ---: | ---: |
| Plaine | 135 | 31 | 13 | 1 | 90 |
| Désert | 145 | 24 | 9 | 0 | 112 |
| Neige | 105 | 14 | 10 | 0 | 81 |
| Rocheux | 110 | 19 | 13 | 0 | 78 |
| **Total** | **495** | **88** | **45** | **1** | **361** |

Le taux de réussite actuel est **17,78 %**. Parmi les 361 parties inachevées, 198 présentent une avance de population bleue et 72 dépassent même le double de la population rouge. Exemple : monde 25, **183 395 contre 9 517**, sans élimination à vingt minutes. L’ancien indicateur « population supérieure sur quelques graines » ne mesurait donc pas la capacité à gagner.

Les données détaillées sont dans [baseline.csv](ai-research/baseline.csv). Le [protocole et les empreintes SHA-256](ai-research/baseline-methodology.md) identifient précisément les sources et données testées. Attention : la colonne `actions` est une borne inférieure de marqueurs observables, pas un total exact ; les événements offensifs agrègent les deux camps. Les comptes de villes/châteaux sont échantillonnés avant les actions du dernier tick.

## Corriger le moteur avant de régler la stratégie

### 1. Les emplacements des groupes morts ne sont pas réutilisés

`spawnWalkerFromTown` bloque dès que `len(Peeps)` atteint 208, même si presque tous les groupes sont morts. Les villes continuent de croître mais ne produisent plus d’émigrants. Les 361 parties inachevées ont toutes atteint cette limite, avec une médiane de **165 emplacements morts**. C’est une forte association, pas la preuve que ce défaut explique à lui seul tous les blocages.

Le [source original réutilise les emplacements morts](https://github.com/LemonHaze420/DCPopulous/blob/master/populous_peeps.cpp#L211-L245). Restaurer ce mécanisme pour **les deux camps** rétablit une règle du jeu ; cela ne constitue pas un avantage caché pour l’IA stratégique.

### 2. Un chevalier peut rester bloqué sur une ville amie

Le Go refuse à juste titre sa fusion avec la ville, puis arrête aussi son déplacement. L’original [laisse poursuivre le personnage qui a survécu au contact](https://github.com/LemonHaze420/DCPopulous/blob/master/populous_peeps.cpp#L405-L440).

Deux tests reproducteurs échouent avec le moteur actuel et passent avec un prototype symétrique. Sur neuf mondes exploratoires, sans changer la stratégie :

| Monde | Moteur actuel | Deux corrections moteur |
| --- | --- | --- |
| 0 | Nul à 699 s | Victoire à 847,875 s |
| 100 | Inachevé | Victoire à 629,250 s |
| 200 | Victoire à 491,375 s | Victoire à 971,375 s |
| 250 | Inachevé | Victoire à 480,375 s |
| 450 | Victoire à 1 072,500 s | Défaite à 955,875 s |

Les mondes 5, 25, 80 et 400 restent inachevés dans les deux versions. Le bilan passe de **2 à 4 victoires sur 9**, mais l’adversaire bénéficie également des corrections. Ce petit panel ne permet pas d’extrapoler un taux sur toute la campagne.

Le [patch expérimental](ai-research/engine-prototype.patch) et les [tests reproducteurs archivés](ai-research/engine_probe_test.go.txt) sont conservés pour examen. Ils ne sont pas appliqués au moteur courant. Leur intégration nécessiterait aussi de revalider les sauvegardes, le déterminisme et la compatibilité entre versions réseau.

### 3. Armageddon doit tenir compte des chemins et du combat

Les traces des mondes 100 et 102 montrent une armée oscillant entre deux cases devant un marais, pendant que l’autre attend au centre. En 512 ticks, l’armée mobile conserve 32 000 habitants, tandis que celle qui attend en perd 4 096.

Deux divergences de port sont à examiner : l’attrition des marcheurs est conditionnée à `InOut != 0` dans le Go, contrairement à sa [soustraction dans l’original](https://github.com/LemonHaze420/DCPopulous/blob/master/populous_peeps.cpp#L278-L281) ; l’évitement des marais est également plus large dans le Go. Revenir littéralement au comportement original pourrait faire entrer une armée dans les marais et la tuer. La stratégie doit donc préparer les trajets, pas seulement déclencher le sort.

Le critère actuel de population globale est insuffisant : les armes comptent dans `doBattle`, et les regroupements dans `joinForces` plafonnent un groupe à 32 000. Une très grande population dispersée peut perdre son avantage lors de la concentration.

## Économie adaptée à chaque terrain

Ces valeurs sont décodées des fichiers `assets/amiga/land0` à `land3` et appliquées une fois par seconde aux villes. Elles excluent les autres gains et dépenses de mana.

| Terrain | Mana/s, grande ville ordinaire (stade 9) | Mana/s, château | Population/s, château | Orientation à tester |
| --- | ---: | ---: | ---: | --- |
| Plaine | 6 | 20 | 5 | Expansion initiale puis châteaux de production |
| Désert | 6 | 6 | 10 | Réseau de villes et troupes regroupées ; château surtout pour population/armement |
| Neige | 3 | 3 | 9 | Densité d’implantations pour la mana, châteaux sélectionnés pour la croissance |
| Rocheux | 2 | 30 | 10 | Quelques châteaux très rentables, expansion autour des roches difficiles |

Un château a une capacité de **3 050** dans ce moteur, contre au plus **290** avant le seuil de transformation. Le terminer trop tôt peut retarder fortement le départ des colons. Ce compromis entre capacité et émigration est aussi décrit dans le [manuel original](https://www.lemonamiga.com/doc/populous/1259).

Il faut donc comparer, pour chaque chantier, le coût des actions, le gain de mana attendu, le délai avant le prochain émigrant, l’espace ouvert pour d’autres villages et le risque de destruction. Le nombre brut de châteaux ne doit plus être l’objectif principal.

Conserver les plateaux utiles à 3 ou 4 ; privilégier l’altitude 2 quand Flood est disponible chez l’adversaire et menace réellement les implantations. Éviter d’abaisser un plateau entier pour enlever une roche dure. Prévoir une réparation après Volcano en fonction des zones encore habitables et du coût, plutôt que restaurer mécaniquement toute la surface.

## Stratégies à développer

### Expansion et émission contrôlée de colons

Commencer par plusieurs villages lorsque les colons manquent, puis promouvoir les centres de production adaptés au terrain. Une alternative légale à tester consiste à réduire temporairement le terrain nourricier d’un château rempli, attendre une émigration normale, puis restaurer le coin modifié. Les deux actions doivent être payées, respecter la cadence et ne pas sacrifier les villes voisines.

Il ne faut pas reproduire le raccourci du C++ qui accorde une émigration anticipée particulière à l’ordinateur en modifiant sa capacité effective : ce serait contraire à l’exigence d’une nouvelle IA sans privilèges.

### Harcèlement avec objectif économique

Choisir une cible selon les revenus et la croissance qu’elle va perdre, la capacité adverse à réparer, la proximité du front et le coût du sort. Éviter de dépenser continuellement en séismes contre des cibles qui survivent sans empêcher la reconstruction.

Une réserve de mana doit avoir un but précis : contrer une menace imminente, préparer un chevalier, ou financer Armageddon lorsque les conditions militaires sont réunies. Épargner systématiquement laisse parfois l’adversaire prendre l’avantage ; dépenser systématiquement empêche de conclure.

### Expéditions avec itinéraire et soutien

`FightMode` recherche des adversaires localement : ce n’est pas un ordre d’invasion globale. Il faut guider le fanion par étapes, créer des points d’appui, regrouper des troupes assez fortes pour survivre à l’attrition et employer les commandes de terrain normales pour ouvrir les passages.

Avant de lancer un chevalier, vérifier qu’il peut rejoindre une cible intéressante. Si le front arrive près d’une ville adverse, le terrassement offensif peut être très économique, mais uniquement lorsqu’une présence de construction valide le permet. Dans les mondes où seules les villes autorisent la construction, établir d’abord un avant-poste.

### Conclusion de partie adaptée aux pouvoirs

**Avec Armageddon :** estimer la force réellement disponible après regroupement, l’armement, la durée des trajets et les obstacles vers le centre. Sécuriser les marais et passages nécessaires avant le déclenchement. Le mana requis reste celui du jeu : 80 000.

**Sans Armageddon :** employer des vagues de chevaliers si autorisées, sinon des expéditions au fanion, des positions avancées et le terrassement offensif légal. Prévoir explicitement la recherche des derniers survivants et le traitement des îles restantes.

**Flood :** évaluer une copie de la carte après la baisse d’altitude déterministe : vies exposées, rendement des villes et chemins coupés des deux camps. La simple population située au niveau bas ne suffit pas.

Le planificateur doit estimer le temps nécessaire pour conclure par rapport aux secondes restantes, et changer de plan lorsque l’armée ne progresse plus. Un chronomètre qui attribue artificiellement la victoire ne résout pas le problème.

## Résultats des variantes exploratoires

Les essais ont conservé mana, pouvoirs, cadence et générateur aléatoire normaux. Les décisions de terrassement expérimental utilisent une prévision déterministe sans lire les futurs tirages de combat ou de sorts. Les panneaux de neuf mondes exploratoires puis dix profils distincts servent à déceler les régressions ; ils ne remplacent pas les 495 validations finales.

- **Épargne Armageddon + colonisation :** transforme le monde 5 en victoire à 994 s et, avec un autre seuil, le monde 250 en victoire à 499 s. Mais transforme aussi la victoire du monde 450 en défaite à 815,5 s. Pas de règle universelle.
- **Villages avant châteaux :** transforme notamment les mondes 35 et 350, initialement inachevés, en victoires à 1 104 s et 526,375 s sur le moteur courant. D’autres cartes régressent.
- **Soutien terrain aux expéditions :** transforme le monde 10, initialement perdu à 476,625 s, en victoire à 608,625 s sur le moteur courant.
- **Même soutien avec les deux corrections moteur :** 5 victoires et 4 parties inachevées sur le panel de neuf, contre 4 victoires, 4 inachevées et 1 défaite pour la stratégie actuelle avec ces mêmes corrections. Signal encourageant, encore très loin d’une validation universelle.
- **Émissions temporaires de château :** premier résultat prometteur sur le monde 200, victoire à 337,250 s au lieu de 971,375 s avec moteur corrigé ; plusieurs régressions ailleurs. Une politique conditionnelle reste nécessaire.

En incluant les dix profils de validation distincts, le prototype de soutien offensif obtient **11 victoires sur 19, 8 parties inachevées et aucune défaite**, contre **9 victoires, 9 inachevées et 1 défaite** pour l’IA actuelle sur le même moteur corrigé. Sur les dix profils réservés seuls : 6 victoires contre 5. Le monde 475 passe de 620,250 à 354,500 secondes ; le monde 10, auparavant inachevé sur ce moteur, termine en 651,375 secondes. Ces 19 profils ne sont pas un échantillon aléatoire : le taux ne doit pas être extrapolé aux 495 mondes. Le [compte rendu complet des variantes](ai-research/strategy-experiments.md) présente aussi les régressions.

Les résultats positifs ne justifient pas de choisir une stratégie mémorisée par numéro de monde. Le choix doit dépendre de propriétés observables : terrain, pouvoirs, ressources, implantation, accessibilité et rapport de forces.

## Critère « sans tricher » et validation finale

1. Aucun ajout de mana, habitants, armes, sorts, actions ou terrain gratuit.
2. Aucune téléportation ni modification directe de la population ennemie.
3. Présence de construction vérifiée à **chaque** action, y compris à la reprise d’un chantier ; ne pas se contenter des gardes partielles de `changeAltitude`.
4. Pas de consultation du prochain tirage du générateur aléatoire. Les essais probabilistes éventuels doivent employer des tirages indépendants, pas prédire l’avenir du moteur.
5. Corrections des règles du moteur appliquées aux deux camps, puis revalidation des effets sur l’adversaire.
6. Chaque monde conserve ses pouvoirs, son terrain, ses ressources initiales et la cadence convenue. Aucun niveau défavorable exclu du bilan.
7. Une victoire exige l’élimination effective avant ou au tick 9 600. Ni avantage économique, ni score, ni limite de temps ne comptent.

Il existe **105 mondes sans Armageddon**, **10 sans aucun sort** et **10 sans construction**. Ces restrictions interdisent une solution reposant sur un seul sort ou uniquement sur l’aménagement du terrain. Les mondes 265–269, départ d’un groupe contre six et Armageddon adverse, perdent actuellement tous en moins de huit minutes : ils doivent faire partie des cas de défense précoce.

Ordre recommandé : corriger et tester les divergences moteur ; recalculer les 495 résultats ; développer un planificateur économique propre aux terrains et des expéditions soutenues ; ajouter une politique de fin de partie ; enfin refaire la campagne entière. La cible vérifiable est **495/495 victoires en vingt minutes avec les règles annoncées**, sans prétendre que quelques bons résultats la démontrent déjà.

## Reproduction et pièces conservées

- [CSV complet des 495 mondes](ai-research/baseline.csv).
- [Méthodologie et empreintes des fichiers](ai-research/baseline-methodology.md).
- [Source du banc exhaustif, archivée comme texte](ai-research/research_audit_test.go.txt) : test `TestResearchCampaignAudit`, activation `POPULOUS_RESEARCH_AUDIT=1`. La simulation peut être relancée en installant temporairement ce fichier de test dans `internal/populous`, ou au moyen d’un overlay Go.
- [Prototype des deux corrections moteur](ai-research/engine-prototype.patch), **non appliqué**.
- [Reproducteurs moteur](ai-research/engine_probe_test.go.txt).
- [Prototypes de stratégies archivés](ai-research/research_strategy_test.go.txt), [résultats du panel corrigé](ai-research/strategy-fixed-panel.txt) et [validation sur dix profils distincts](ai-research/strategy-fixed-validation.txt).

Les données brutes plus volumineuses, snapshots et traces des Armageddons bloqués restent dans `/private/tmp/populous-ai-research`. Les variantes de stratégie et leurs journaux sont dans `/private/tmp/populous-strategy-lab` ; les overlays moteur dans `/private/tmp/populous-engine-audit.YvJITN`. Ces chemins temporaires sont des artefacts de diagnostic, pas des dépendances de l’application.
