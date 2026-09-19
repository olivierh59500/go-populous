# Interface tactile et affichage élargi

La présentation mobile est activée par `SetMobileUI(true)` dans le bridge
Android. Le bureau conserve sa présentation originale, sauf lancement avec
`-mobile-ui` pour prévisualiser la même interface. La nouvelle présentation
concerne la partie, ses outils et pouvoirs, le menu de pause ainsi que
l’accueil et les menus de préparation. Le rendu du mode démo reste indépendant
et inchangé.

## Gestes et outils

- Un doigt : sélectionner un outil ou viser une case ; l’action est validée
  au relâchement, puis transmise au prochain tick de simulation.
- Deux doigts démarrant sur le terrain : déplacer naturellement la carte.
  Le relâchement d’un seul doigt ne déclenche pas d’action avec l’autre.
- Un mélange terrain/interface, un troisième doigt ou une annulation de
  cycle de vie annule le geste jusqu’au relâchement complet.
- Les outils Monter, Descendre, Fanion et Inspection sont persistants.
  Maintenir un doigt ne répète pas le terrassement : il affiche la cible.
- Le panneau des pouvoirs indique coûts et disponibilité. Séisme, marais et
  volcan demandent une cible ; Flood et Armageddon une confirmation explicite.
- Le menu donne accès à la sauvegarde, au chargement, à l’aide, au titre et
  aux réglages. Le pad directionnel reste disponible en option.

Android échantillonne toujours à 60 Hz ; la simulation reste à 8 Hz. La caméra
et les panneaux répondent au rythme de l’interface. Les commandes sont mises
en file pour le tick ; les changements de menu, de monde ou de cycle de vie
annulent les commandes en attente. `Mobile.cancelInput()` utilise un signal
atomique consommé par la boucle Go, avant de lire les nouveaux contacts.

## Réglages de construction

`MENU` → `SETTINGS` → `BUILD AREA` propose :

- **LOCAL 8x8**, valeur par défaut : présence amie dans un voisinage 8×8
  centré près de la case ciblée, décalé pour rester dans la carte aux bords.
  Sa taille ne dépend pas de la largeur physique de l’écran. Il s’agit d’une
  règle locale explicite, et non du rectangle exact de l’ancienne caméra.
- **VISIBLE MAP** : présence amie ailleurs dans le terrain affiché. Les
  parties couvertes par la minicarte ou le pad ne sont pas prises en compte.
  Ce choix est une option de jeu solo volontairement plus permissive.

Les restrictions du monde (construction interdite, villes seules, montée
seulement) et les coûts continuent d’être appliqués. L’option ne change ni la
taille des sorts ni leur coût. En multijoueur, la portée locale est imposée et
le réglage est verrouillé ; l’option étendue n’est pas négociée sur le réseau.

`WIDE TERRAIN` active l’affichage de davantage de terrain ; le désactiver
limite la vue à 8×8 cases. `VIRTUAL PAD` active les boutons directionnels.
Les réglages sont sauvegardés atomiquement dans `go-populous-ui.json`, à côté
de la sauvegarde du jeu, dans le stockage privé sur Android. Les anciennes
sauvegardes de monde restent compatibles.

## Accueil et préparation tactile

L’accueil présente huit grands boutons : tutoriel, conquête, partie
personnalisée, configuration, réglages tactiles, chargement, aide et démo.
Les cibles tactiles mesurent au moins 32 pixels logiques par axe et les
appuis sont validés au relâchement sur le même bouton ; les interstices ne
déclenchent pas de choix.

La conquête affiche le monde et propose des pas de ±1 ou ±10 avant de jouer.
La configuration et les options personnalisées sont paginées. Les valeurs
ont de grands boutons moins/plus ; les six pouvoirs se règlent séparément
pour les camps bleu et rouge. Un bouton PLAY est présent sur chaque page
personnalisée. L’aide tactile est également paginée, et les réglages d’affichage
sont accessibles directement depuis l’accueil.

La démo automatique ne démarre que depuis l’accueil, pas pendant la préparation
d’un monde ou la lecture de l’aide. Le bouton DEMO appelle le mode existant.
Le contact servant à quitter une démo est consommé et ne sélectionne pas un
bouton de l’accueil au relâchement.

Un contrôle graphique optionnel est disponible avec une session graphique :

```sh
go run -tags frontmenucheck ./cmd/front-menu-check
```

Lors du contrôle initial, 126 assertions d’intégration ont validé les choix,
les transitions, la pagination, les réglages par camp, la persistance des
préférences, l’inactivité et la consommation des gestes. Une image de démo
recalculée avec puis sans interface mobile a donné des pixels strictement
identiques et les mêmes dimensions 648×296. Les fichiers `demo.go`,
`demo_recording.go` et la session de démo sont inchangés. Le contrôle a ensuite
été étendu au chargement d’une sauvegarde privée ; cette extension est compilée
mais n’a pas pu être réexécutée après la fermeture de la session graphique.

## Architecture et validation

- `internal/mobileui` contient les gestes, disposition, projection, préférences
  et vérifications de construction sans dépendance graphique.
- Le rendu mobile emploie la même carte 64×64, les mêmes images et personnages,
  mais une projection sans les anciens retours de coordonnées à 320 pixels.
  Rendu et sélection utilisent les mêmes bornes, altitudes et ordre des tuiles.
- Le cache de terrain est invalidé par un tick ou un déplacement de caméra.
  Les aperçus de cible et les panneaux restent indépendants de ce cache.
- Les empreintes des sources de simulation et d’IA n’ont pas changé pendant
  cette refonte : aucune modification de `internal/populous` n’était nécessaire.

Tests exécutés : gestes interrompus et réutilisation des identifiants, différence
60/8 Hz par conservation des commandes, géométrie au-delà de 8 cases, hauteur,
frontières, réglages persistants, présence alliée et invariance du hash du monde.
Un contrôle dans une vraie boucle Ebitengine vérifie le pan sans mutation du
monde, le paiement des actions au tick, l’isolation des panneaux, la restauration
des préférences, l’annulation de Flood et le mode bureau par défaut.

Les captures de contrôle sont dans `/private/tmp/populous-mobile-game.png`,
`populous-mobile-powers.png`, `populous-mobile-settings.png` et
`populous-mobile-classic.png`. Les tests Go et le détecteur de races des modules
tactiles passent, ainsi que `go vet ./...`.

L’APK arm64 a été recompilé et installé sur le Pixel 10a, avec signature et
alignement 16 KiB vérifiés. La sauvegarde existante est conservée. Le téléphone
étant verrouillé, le ressenti réel des gestes sur cet appareil reste à tester
après déverrouillage ; il n’a pas été déverrouillé automatiquement.

## Correction des personnages recouverts par le terrain

Le rendu mobile dessinait un personnage immédiatement après sa case source.
Pendant son déplacement, son sprite empiète déjà sur les cases suivantes,
qui pouvaient alors recouvrir ses jambes, voire tout le personnage. Les
coordonnées et l’altitude correspondaient pourtant au rendu bureau.

Le rendu emploie maintenant deux passes : terrain, objets et murs d’abord,
puis personnages, fanions portés et boucliers. Les positions et la simulation
restent inchangées. C’est le même principe de visibilité que sur le bureau ;
il ne s’agit pas d’une nouvelle gestion d’occultation physique des personnages
derrière les montagnes.

Le contrôle graphique reproductible nécessite une session graphique :

```sh
go run -tags rendercheck ./cmd/rendercheck -screenshot /tmp/populous-rendercheck.png
```

Il compare le rendu GPU à une composition indépendante des sprites sur le CPU,
sur quatre terrains, les fermes, les pentes, huit directions, sept étapes de
marche, les états immobiles, chevaliers, eau, batailles, effets et marqueurs,
ainsi que le clipping et la restriction 8×8. Les **774 cas passent** avec la
correction ; **695 échouent** avec l’ancien rendu. Chaque cas vérifie que le
hash du monde reste inchangé. Ces outils marqués `rendercheck` ne sont pas
inclus dans les exécutables ordinaires ou l’APK.
