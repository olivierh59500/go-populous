package com.olivierh.populous;

import android.Manifest;
import android.annotation.SuppressLint;
import android.app.Activity;
import android.app.AlertDialog;
import android.bluetooth.BluetoothAdapter;
import android.bluetooth.BluetoothDevice;
import android.bluetooth.BluetoothManager;
import android.bluetooth.BluetoothServerSocket;
import android.bluetooth.BluetoothSocket;
import android.content.BroadcastReceiver;
import android.content.Context;
import android.content.Intent;
import android.content.IntentFilter;
import android.content.pm.PackageManager;
import android.os.Build;
import android.widget.ArrayAdapter;

import com.olivierh.populous.mobile.Mobile;

import java.io.IOException;
import java.io.InputStream;
import java.io.OutputStream;
import java.net.InetAddress;
import java.net.InetSocketAddress;
import java.net.ServerSocket;
import java.net.Socket;
import java.net.SocketTimeoutException;
import java.security.MessageDigest;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.Collections;
import java.util.HashMap;
import java.util.HashSet;
import java.util.List;
import java.util.Map;
import java.util.Set;
import java.util.UUID;
import java.util.concurrent.atomic.AtomicBoolean;

/**
 * Secure Bluetooth Classic transport for Populous multiplayer.
 *
 * <p>The Go multiplayer implementation continues to use ordinary TCP streams. The host side
 * accepts RFCOMM and connects it to the already-running Go loopback listener; the joining side
 * exposes its RFCOMM stream through a temporary loopback listener and tells Go where to connect.
 * Game data is copied byte-for-byte and is never logged.</p>
 */
final class BluetoothMultiplayer implements AutoCloseable {
    private static final String SERVICE_NAME = "Populous Multiplayer";
    private static final UUID SERVICE_UUID =
            UUID.fromString("504f5055-4c4f-5553-4d50-000000000001");
    private static final int DISCOVERABLE_SECONDS = 180;
    private static final int LOOPBACK_CONNECT_TIMEOUT_MS = 5_000;
    private static final int LOOPBACK_ACCEPT_TIMEOUT_MS = 30_000;

    private enum Action {
        NONE,
        HOST,
        JOIN
    }

    private enum ActivityStage {
        ENABLE,
        DISCOVERABLE
    }

    private static final class ActivityRequest {
        final long generation;
        final ActivityStage stage;

        ActivityRequest(long generation, ActivityStage stage) {
            this.generation = generation;
            this.stage = stage;
        }
    }

    private static final class DeviceEntry {
        final BluetoothDevice device;
        final String label;

        DeviceEntry(BluetoothDevice device, String label) {
            this.device = device;
            this.label = label;
        }
    }

    private final Activity activity;
    private final BluetoothAdapter adapter;
    private final Object lock = new Object();
    private final Map<Integer, Long> permissionRequests = new HashMap<>();
    private final Map<Integer, ActivityRequest> activityRequests = new HashMap<>();

    private long generation;
    private int nextRequestCode = 4_100;
    private boolean destroyed;
    private Action pendingAction = Action.NONE;
    private int pendingHostPort;
    private byte[] pendingToken;

    private BluetoothServerSocket rfcommServer;
    private BluetoothSocket rfcommSocket;
    private ServerSocket loopbackServer;
    private Socket loopbackSocket;
    private Thread worker;
    private Bridge bridge;

    // Discovery UI is only touched on the Activity's main thread.
    private BroadcastReceiver discoveryReceiver;
    private AlertDialog deviceDialog;
    private ArrayAdapter<String> deviceListAdapter;
    private final List<DeviceEntry> deviceEntries = new ArrayList<>();
    private final Set<String> deviceAddresses = new HashSet<>();

    BluetoothMultiplayer(Activity activity) {
        this.activity = activity;
        BluetoothManager manager =
                (BluetoothManager) activity.getSystemService(Context.BLUETOOTH_SERVICE);
        this.adapter = manager == null ? null : manager.getAdapter();
    }

    boolean isAvailable() {
        return adapter != null;
    }

    void handleCommand(String command) {
        if (command == null) {
            return;
        }
        if (command.equals("BT_CANCEL")) {
            cancel();
            return;
        }
        if (command.startsWith("BT_JOIN|")) {
            try {
                String[] fields = command.split("\\|", -1);
                if (fields.length != 2 || !fields[0].equals("BT_JOIN")) {
                    throw new IllegalArgumentException("invalid join command");
                }
                begin(Action.JOIN, 0, parseToken(fields[1]));
            } catch (IllegalArgumentException error) {
                reportFailureForCurrent("Invalid Bluetooth join command");
            }
            return;
        }
        if (command.startsWith("BT_HOST|")) {
            try {
                String[] fields = command.split("\\|", -1);
                if (fields.length != 3 || !fields[0].equals("BT_HOST")) {
                    throw new IllegalArgumentException("invalid host command");
                }
                int port = Integer.parseInt(fields[1]);
                if (port < 1 || port > 65_535) {
                    throw new NumberFormatException("port outside valid range");
                }
                begin(Action.HOST, port, parseToken(fields[2]));
            } catch (IllegalArgumentException error) {
                reportFailureForCurrent("Invalid Bluetooth host command");
            }
            return;
        }
        reportFailureForCurrent("Unknown Bluetooth command");
    }

    private static byte[] parseToken(String encoded) {
        if (encoded == null || encoded.length() != 32) {
            throw new IllegalArgumentException("invalid token length");
        }
        byte[] decoded = new byte[16];
        for (int index = 0; index < decoded.length; index++) {
            int high = asciiHexDigit(encoded.charAt(index * 2));
            int low = asciiHexDigit(encoded.charAt(index * 2 + 1));
            if (high < 0 || low < 0) {
                throw new IllegalArgumentException("invalid token encoding");
            }
            decoded[index] = (byte) ((high << 4) | low);
        }
        return decoded;
    }

    private static int asciiHexDigit(char value) {
        if (value >= '0' && value <= '9') {
            return value - '0';
        }
        if (value >= 'a' && value <= 'f') {
            return value - 'a' + 10;
        }
        if (value >= 'A' && value <= 'F') {
            return value - 'A' + 10;
        }
        return -1;
    }

    private void begin(Action action, int hostPort, byte[] tokenBytes) {
        final long token;
        synchronized (lock) {
            if (destroyed) {
                Arrays.fill(tokenBytes, (byte) 0);
                return;
            }
            generation++;
            closeResourcesLocked();
            pendingAction = action;
            pendingHostPort = hostPort;
            pendingToken = tokenBytes.clone();
            token = generation;
        }
        Arrays.fill(tokenBytes, (byte) 0);

        if (adapter == null) {
            fail(token, "Bluetooth is not available on this device");
            return;
        }
        requestPermissionsOrContinue(token);
    }

    private void cancel() {
        final long token;
        synchronized (lock) {
            if (destroyed) {
                return;
            }
            generation++;
            closeResourcesLocked();
            pendingAction = Action.NONE;
            pendingHostPort = 0;
            clearPendingTokenLocked();
            token = generation;
        }
        reportStatus(token, "Bluetooth cancelled");
    }

    @SuppressLint("MissingPermission")
    private void requestPermissionsOrContinue(long token) {
        String[] missing = missingPermissions();
        if (missing.length == 0) {
            continueAfterPermissions(token);
            return;
        }

        int requestCode;
        synchronized (lock) {
            if (!isCurrentLocked(token)) {
                return;
            }
            requestCode = allocateRequestCodeLocked();
            permissionRequests.put(requestCode, token);
        }
        reportStatus(token, "Bluetooth: requesting permission");
        activity.requestPermissions(missing, requestCode);
    }

    void onRequestPermissionsResult(int requestCode) {
        final Long token;
        synchronized (lock) {
            token = permissionRequests.remove(requestCode);
            if (token == null || !isCurrentLocked(token)) {
                return;
            }
        }
        if (missingPermissions().length != 0) {
            fail(token, "Bluetooth permission was denied");
            return;
        }
        continueAfterPermissions(token);
    }

    @SuppressLint("MissingPermission")
    private void continueAfterPermissions(long token) {
        synchronized (lock) {
            if (!isCurrentLocked(token)) {
                return;
            }
        }
        if (!adapter.isEnabled()) {
            int requestCode;
            synchronized (lock) {
                if (!isCurrentLocked(token)) {
                    return;
                }
                requestCode = allocateRequestCodeLocked();
                activityRequests.put(
                        requestCode, new ActivityRequest(token, ActivityStage.ENABLE));
            }
            reportStatus(token, "Bluetooth: waiting to be enabled");
            activity.startActivityForResult(
                    new Intent(BluetoothAdapter.ACTION_REQUEST_ENABLE), requestCode);
            return;
        }
        continueWithEnabledAdapter(token);
    }

    private void continueWithEnabledAdapter(long token) {
        final Action action;
        synchronized (lock) {
            if (!isCurrentLocked(token)) {
                return;
            }
            action = pendingAction;
        }
        if (action == Action.HOST) {
            requestDiscoverability(token);
        } else if (action == Action.JOIN) {
            showDeviceChooser(token);
        }
    }

    @SuppressLint("MissingPermission")
    private void requestDiscoverability(long token) {
        int requestCode;
        synchronized (lock) {
            if (!isCurrentLocked(token)) {
                return;
            }
            requestCode = allocateRequestCodeLocked();
            activityRequests.put(
                    requestCode, new ActivityRequest(token, ActivityStage.DISCOVERABLE));
        }
        Intent intent = new Intent(BluetoothAdapter.ACTION_REQUEST_DISCOVERABLE);
        intent.putExtra(BluetoothAdapter.EXTRA_DISCOVERABLE_DURATION, DISCOVERABLE_SECONDS);
        reportStatus(token, "Bluetooth: requesting visibility");
        activity.startActivityForResult(intent, requestCode);
    }

    void onActivityResult(int requestCode, int resultCode) {
        final ActivityRequest request;
        synchronized (lock) {
            request = activityRequests.remove(requestCode);
            if (request == null || !isCurrentLocked(request.generation)) {
                return;
            }
        }
        if (request.stage == ActivityStage.ENABLE) {
            if (resultCode != Activity.RESULT_OK || !isAdapterEnabled()) {
                fail(request.generation, "Bluetooth was not enabled");
                return;
            }
            continueWithEnabledAdapter(request.generation);
            return;
        }
        // ACTION_REQUEST_DISCOVERABLE returns the granted duration as a positive result.
        if (resultCode <= 0) {
            fail(request.generation, "Bluetooth visibility was declined");
            return;
        }
        startHost(request.generation);
    }

    @SuppressLint("MissingPermission")
    private boolean isAdapterEnabled() {
        try {
            return adapter != null && adapter.isEnabled();
        } catch (SecurityException error) {
            return false;
        }
    }

    private String[] missingPermissions() {
        final Action action;
        synchronized (lock) {
            action = pendingAction;
        }
        List<String> permissions = new ArrayList<>();
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.S) {
            addIfMissing(permissions, Manifest.permission.BLUETOOTH_CONNECT);
            if (action == Action.HOST) {
                addIfMissing(permissions, Manifest.permission.BLUETOOTH_ADVERTISE);
            } else if (action == Action.JOIN) {
                addIfMissing(permissions, Manifest.permission.BLUETOOTH_SCAN);
            }
        } else if (action == Action.JOIN) {
            addIfMissing(permissions, Manifest.permission.ACCESS_FINE_LOCATION);
        }
        return permissions.toArray(new String[0]);
    }

    private void addIfMissing(List<String> permissions, String permission) {
        if (activity.checkSelfPermission(permission) != PackageManager.PERMISSION_GRANTED) {
            permissions.add(permission);
        }
    }

    @SuppressLint("MissingPermission")
    private void startHost(long token) {
        final int port;
        final byte[] proxyToken;
        synchronized (lock) {
            if (!isCurrentLocked(token) || pendingAction != Action.HOST) {
                return;
            }
            port = pendingHostPort;
            proxyToken = pendingToken == null ? null : pendingToken.clone();
        }
        if (proxyToken == null || proxyToken.length != 16) {
            fail(token, "Bluetooth proxy authentication is unavailable");
            return;
        }
        startWorker(token, "Populous-Bluetooth-Host", () -> {
            BluetoothServerSocket server = null;
            BluetoothSocket bluetooth = null;
            Socket local = null;
            try {
                server = adapter.listenUsingRfcommWithServiceRecord(SERVICE_NAME, SERVICE_UUID);
                if (!publishRfcommServer(token, server)) {
                    closeQuietly(server);
                    return;
                }
                reportStatus(token, "Bluetooth: waiting for another player");
                bluetooth = server.accept();
                clearRfcommServer(server);
                closeQuietly(server);
                server = null;
                if (!publishRfcommSocket(token, bluetooth)) {
                    closeQuietly(bluetooth);
                    return;
                }

                local = new Socket();
                local.connect(
                        new InetSocketAddress(InetAddress.getByName("127.0.0.1"), port),
                        LOOPBACK_CONNECT_TIMEOUT_MS);
                if (!publishLoopbackSocket(token, local)) {
                    closeQuietly(local);
                    return;
                }
                OutputStream localOutput = local.getOutputStream();
                localOutput.write(proxyToken);
                localOutput.flush();
                clearPendingToken(token);
                String peerName = safeDeviceName(bluetooth.getRemoteDevice());
                startBridge(token, bluetooth, local, peerName);
            } catch (IOException | SecurityException error) {
                closeQuietly(server);
                closeQuietly(bluetooth);
                closeQuietly(local);
                failIfCurrent(token, "Bluetooth host connection failed");
            } finally {
                Arrays.fill(proxyToken, (byte) 0);
            }
        });
    }

    @SuppressLint("MissingPermission")
    private void showDeviceChooser(long token) {
        synchronized (lock) {
            if (!isCurrentLocked(token) || pendingAction != Action.JOIN) {
                return;
            }
        }
        stopDiscoveryUi();
        deviceEntries.clear();
        deviceAddresses.clear();

        List<BluetoothDevice> bonded = new ArrayList<>(adapter.getBondedDevices());
        Collections.sort(bonded, (left, right) ->
                safeDeviceName(left).compareToIgnoreCase(safeDeviceName(right)));
        for (BluetoothDevice device : bonded) {
            addDevice(device, true);
        }

        deviceListAdapter = new ArrayAdapter<>(
                activity, android.R.layout.simple_list_item_1, new ArrayList<>());
        refreshDeviceLabels();
        AlertDialog dialog = new AlertDialog.Builder(activity)
                .setTitle("Join Bluetooth game")
                .setAdapter(deviceListAdapter, (ignored, which) -> {
                    if (which < 0 || which >= deviceEntries.size()) {
                        return;
                    }
                    BluetoothDevice selected = deviceEntries.get(which).device;
                    stopDiscoveryUi();
                    connectToDevice(token, selected);
                })
                .setNegativeButton(android.R.string.cancel, (ignored, which) ->
                        fail(token, "Bluetooth device selection was cancelled"))
                .create();
        dialog.setOnCancelListener(ignored ->
                fail(token, "Bluetooth device selection was cancelled"));
        deviceDialog = dialog;

        discoveryReceiver = new BroadcastReceiver() {
            @Override
            public void onReceive(Context context, Intent intent) {
                synchronized (lock) {
                    if (!isCurrentLocked(token)) {
                        return;
                    }
                }
                String action = intent.getAction();
                if (BluetoothDevice.ACTION_FOUND.equals(action)) {
                    BluetoothDevice device;
                    if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
                        device = intent.getParcelableExtra(
                                BluetoothDevice.EXTRA_DEVICE, BluetoothDevice.class);
                    } else {
                        device = intent.getParcelableExtra(BluetoothDevice.EXTRA_DEVICE);
                    }
                    if (device != null && addDevice(device, false)) {
                        refreshDeviceLabels();
                    }
                } else if (BluetoothAdapter.ACTION_DISCOVERY_FINISHED.equals(action)) {
                    reportStatus(token, deviceEntries.isEmpty()
                            ? "Bluetooth: no devices found"
                            : "Bluetooth: select a device");
                }
            }
        };
        IntentFilter filter = new IntentFilter();
        filter.addAction(BluetoothDevice.ACTION_FOUND);
        filter.addAction(BluetoothAdapter.ACTION_DISCOVERY_FINISHED);
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
            activity.registerReceiver(discoveryReceiver, filter, Context.RECEIVER_EXPORTED);
        } else {
            activity.registerReceiver(discoveryReceiver, filter);
        }

        dialog.show();
        reportStatus(token, "Bluetooth: searching for players");
        if (!adapter.startDiscovery()) {
            reportStatus(token, deviceEntries.isEmpty()
                    ? "Bluetooth discovery could not start"
                    : "Bluetooth: select a paired device");
        }
    }

    @SuppressLint("MissingPermission")
    private boolean addDevice(BluetoothDevice device, boolean paired) {
        String address;
        try {
            address = device.getAddress();
        } catch (SecurityException error) {
            return false;
        }
        if (address == null || !deviceAddresses.add(address)) {
            return false;
        }
        String name = safeDeviceName(device);
        String kind = paired ? "[PAIRED] " : "[NEARBY] ";
        deviceEntries.add(new DeviceEntry(device, kind + name + "\n" + address));
        return true;
    }

    private void refreshDeviceLabels() {
        if (deviceListAdapter == null) {
            return;
        }
        deviceListAdapter.clear();
        for (DeviceEntry entry : deviceEntries) {
            deviceListAdapter.add(entry.label);
        }
        deviceListAdapter.notifyDataSetChanged();
    }

    @SuppressLint("MissingPermission")
    private void connectToDevice(long token, BluetoothDevice device) {
        stopDiscoveryUi();
        final String peerName = safeDeviceName(device);
        final byte[] proxyToken;
        synchronized (lock) {
            if (!isCurrentLocked(token)) {
                return;
            }
            proxyToken = pendingToken == null ? null : pendingToken.clone();
        }
        if (proxyToken == null || proxyToken.length != 16) {
            fail(token, "Bluetooth proxy authentication is unavailable");
            return;
        }
        reportStatus(token, "Bluetooth: connecting to " + peerName);
        startWorker(token, "Populous-Bluetooth-Join", () -> {
            BluetoothSocket bluetooth = null;
            ServerSocket localServer = null;
            Socket local = null;
            try {
                bluetooth = device.createRfcommSocketToServiceRecord(SERVICE_UUID);
                if (!publishRfcommSocket(token, bluetooth)) {
                    closeQuietly(bluetooth);
                    return;
                }
                bluetooth.connect();

                localServer = new ServerSocket();
                localServer.setReuseAddress(false);
                localServer.bind(
                        new InetSocketAddress(InetAddress.getByName("127.0.0.1"), 0), 1);
                localServer.setSoTimeout(LOOPBACK_ACCEPT_TIMEOUT_MS);
                if (!publishLoopbackServer(token, localServer)) {
                    closeQuietly(localServer);
                    return;
                }
                int port = localServer.getLocalPort();
                reportClientReady(token, "127.0.0.1:" + port, peerName);
                local = localServer.accept();
                clearLoopbackServer(localServer);
                closeQuietly(localServer);
                localServer = null;
                if (!publishLoopbackSocket(token, local)) {
                    closeQuietly(local);
                    return;
                }
                local.setSoTimeout(LOOPBACK_CONNECT_TIMEOUT_MS);
                byte[] suppliedToken = new byte[proxyToken.length];
                readExactly(local.getInputStream(), suppliedToken);
                boolean authenticated = MessageDigest.isEqual(proxyToken, suppliedToken);
                Arrays.fill(suppliedToken, (byte) 0);
                if (!authenticated) {
                    throw new IOException("loopback proxy authentication failed");
                }
                local.setSoTimeout(0);
                clearPendingToken(token);
                startBridge(token, bluetooth, local, peerName);
            } catch (SocketTimeoutException error) {
                closeQuietly(localServer);
                closeQuietly(local);
                closeQuietly(bluetooth);
                failIfCurrent(token, "The game did not open its Bluetooth connection in time");
            } catch (IOException | SecurityException error) {
                closeQuietly(localServer);
                closeQuietly(local);
                closeQuietly(bluetooth);
                failIfCurrent(token, "Bluetooth connection failed");
            } finally {
                Arrays.fill(proxyToken, (byte) 0);
            }
        });
    }

    private static void readExactly(InputStream input, byte[] destination) throws IOException {
        int offset = 0;
        while (offset < destination.length) {
            int count = input.read(destination, offset, destination.length - offset);
            if (count < 0) {
                throw new IOException("incomplete loopback proxy authentication");
            }
            if (count > 0) {
                offset += count;
            }
        }
    }

    private void clearPendingToken(long token) {
        synchronized (lock) {
            if (isCurrentLocked(token)) {
                clearPendingTokenLocked();
            }
        }
    }

    private void clearPendingTokenLocked() {
        if (pendingToken != null) {
            Arrays.fill(pendingToken, (byte) 0);
            pendingToken = null;
        }
    }

    @SuppressLint("MissingPermission")
    private String safeDeviceName(BluetoothDevice device) {
        try {
            String name = device == null ? null : device.getName();
            return cleanDeviceName(name);
        } catch (SecurityException error) {
            return "Bluetooth player";
        }
    }

    private static String cleanDeviceName(String value) {
        if (value == null) {
            return "Bluetooth player";
        }
        StringBuilder result = new StringBuilder();
        boolean pendingSpace = false;
        int codePoints = 0;
        for (int offset = 0; offset < value.length() && codePoints < 64; ) {
            int codePoint = value.codePointAt(offset);
            offset += Character.charCount(codePoint);
            if (Character.isISOControl(codePoint) || Character.isWhitespace(codePoint)) {
                pendingSpace = result.length() > 0;
                continue;
            }
            if (pendingSpace) {
                result.append(' ');
                pendingSpace = false;
            }
            result.appendCodePoint(codePoint);
            codePoints++;
        }
        return result.length() == 0 ? "Bluetooth player" : result.toString();
    }

    private void startWorker(long token, String name, Runnable task) {
        Thread thread = new Thread(() -> {
            try {
                task.run();
            } finally {
                synchronized (lock) {
                    if (worker == Thread.currentThread()) {
                        worker = null;
                    }
                }
            }
        }, name);
        synchronized (lock) {
            if (!isCurrentLocked(token)) {
                return;
            }
            worker = thread;
        }
        thread.start();
    }

    private boolean publishRfcommServer(long token, BluetoothServerSocket socket) {
        synchronized (lock) {
            if (!isCurrentLocked(token)) {
                return false;
            }
            rfcommServer = socket;
            return true;
        }
    }

    private void clearRfcommServer(BluetoothServerSocket socket) {
        synchronized (lock) {
            if (rfcommServer == socket) {
                rfcommServer = null;
            }
        }
    }

    private boolean publishRfcommSocket(long token, BluetoothSocket socket) {
        synchronized (lock) {
            if (!isCurrentLocked(token)) {
                return false;
            }
            rfcommSocket = socket;
            return true;
        }
    }

    private boolean publishLoopbackServer(long token, ServerSocket socket) {
        synchronized (lock) {
            if (!isCurrentLocked(token)) {
                return false;
            }
            loopbackServer = socket;
            return true;
        }
    }

    private void clearLoopbackServer(ServerSocket socket) {
        synchronized (lock) {
            if (loopbackServer == socket) {
                loopbackServer = null;
            }
        }
    }

    private boolean publishLoopbackSocket(long token, Socket socket) {
        synchronized (lock) {
            if (!isCurrentLocked(token)) {
                return false;
            }
            loopbackSocket = socket;
            return true;
        }
    }

    private void startBridge(
            long token, BluetoothSocket bluetooth, Socket local, String peerName) {
        Bridge newBridge = new Bridge(token, bluetooth, local);
        synchronized (lock) {
            if (!isCurrentLocked(token)) {
                newBridge.close(false);
                return;
            }
            bridge = newBridge;
        }
        reportStatus(token, "Bluetooth connected to " + peerName);
        newBridge.start();
    }

    private final class Bridge {
        private final long token;
        private final BluetoothSocket bluetooth;
        private final Socket local;
        private final AtomicBoolean finished = new AtomicBoolean();
        private Thread bluetoothToLocal;
        private Thread localToBluetooth;

        Bridge(long token, BluetoothSocket bluetooth, Socket local) {
            this.token = token;
            this.bluetooth = bluetooth;
            this.local = local;
        }

        void start() {
            bluetoothToLocal = new Thread(
                    () -> pump(bluetooth, local), "Populous-Bluetooth-To-Local");
            localToBluetooth = new Thread(
                    () -> pump(local, bluetooth), "Populous-Local-To-Bluetooth");
            bluetoothToLocal.start();
            localToBluetooth.start();
        }

        private void pump(BluetoothSocket source, Socket destination) {
            try {
                copy(source.getInputStream(), destination.getOutputStream());
            } catch (IOException ignored) {
                // Closing either side is the normal way a game ends or is cancelled.
            } finally {
                close(true);
            }
        }

        private void pump(Socket source, BluetoothSocket destination) {
            try {
                copy(source.getInputStream(), destination.getOutputStream());
            } catch (IOException ignored) {
                // Closing either side is the normal way a game ends or is cancelled.
            } finally {
                close(true);
            }
        }

        private void copy(InputStream input, OutputStream output) throws IOException {
            byte[] buffer = new byte[16 * 1024];
            while (true) {
                int count = input.read(buffer);
                if (count < 0) {
                    return;
                }
                if (count == 0) {
                    continue;
                }
                output.write(buffer, 0, count);
                output.flush();
            }
        }

        void close(boolean notify) {
            if (!finished.compareAndSet(false, true)) {
                return;
            }
            closeQuietly(bluetooth);
            closeQuietly(local);
            if (bluetoothToLocal != null) {
                bluetoothToLocal.interrupt();
            }
            if (localToBluetooth != null) {
                localToBluetooth.interrupt();
            }
            if (notify) {
                bridgeFinished(token, this);
            }
        }
    }

    private void bridgeFinished(long token, Bridge endedBridge) {
        activity.runOnUiThread(() -> {
            synchronized (lock) {
                if (!isCurrentLocked(token) || bridge != endedBridge) {
                    return;
                }
                bridge = null;
                rfcommSocket = null;
                loopbackSocket = null;
                pendingAction = Action.NONE;
                clearPendingTokenLocked();
            }
            reportStatus(token, "Bluetooth disconnected");
        });
    }

    private int allocateRequestCodeLocked() {
        int result = nextRequestCode++;
        if (nextRequestCode > 60_000) {
            nextRequestCode = 4_100;
        }
        return result;
    }

    private boolean isCurrentLocked(long token) {
        return !destroyed && token == generation;
    }

    private void failIfCurrent(long token, String message) {
        synchronized (lock) {
            if (!isCurrentLocked(token)) {
                return;
            }
        }
        fail(token, message);
    }

    private void fail(long token, String message) {
        activity.runOnUiThread(() -> {
            synchronized (lock) {
                if (!isCurrentLocked(token)) {
                    return;
                }
                closeResourcesLocked();
                pendingAction = Action.NONE;
                pendingHostPort = 0;
                clearPendingTokenLocked();
            }
            try {
                Mobile.bluetoothFailed(message);
            } catch (Exception ignored) {
                // The Activity may already be leaving while Go tears down.
            }
        });
    }

    private void reportFailureForCurrent(String message) {
        final long token;
        synchronized (lock) {
            token = generation;
        }
        fail(token, message);
    }

    private void reportStatus(long token, String message) {
        activity.runOnUiThread(() -> {
            synchronized (lock) {
                if (!isCurrentLocked(token)) {
                    return;
                }
            }
            try {
                Mobile.bluetoothStatus(message);
            } catch (Exception ignored) {
                // The Activity may already be leaving while Go tears down.
            }
        });
    }

    private void reportClientReady(long token, String address, String name) {
        activity.runOnUiThread(() -> {
            synchronized (lock) {
                if (!isCurrentLocked(token)) {
                    return;
                }
            }
            try {
                Mobile.bluetoothClientReady(address, name);
            } catch (Exception error) {
                fail(token, "The game could not start its Bluetooth connection");
            }
        });
    }

    @SuppressLint("MissingPermission")
    private void stopDiscoveryUi() {
        try {
            if (adapter != null && adapter.isDiscovering()) {
                adapter.cancelDiscovery();
            }
        } catch (SecurityException ignored) {
            // Permission can be revoked while the chooser is open.
        }
        if (discoveryReceiver != null) {
            try {
                activity.unregisterReceiver(discoveryReceiver);
            } catch (IllegalArgumentException ignored) {
                // It was already unregistered during Activity teardown.
            }
            discoveryReceiver = null;
        }
        if (deviceDialog != null) {
            deviceDialog.dismiss();
            deviceDialog = null;
        }
        deviceListAdapter = null;
        deviceEntries.clear();
        deviceAddresses.clear();
    }

    private void closeResourcesLocked() {
        stopDiscoveryUi();
        if (bridge != null) {
            bridge.close(false);
            bridge = null;
        }
        closeQuietly(rfcommServer);
        rfcommServer = null;
        closeQuietly(rfcommSocket);
        rfcommSocket = null;
        closeQuietly(loopbackServer);
        loopbackServer = null;
        closeQuietly(loopbackSocket);
        loopbackSocket = null;
        if (worker != null) {
            worker.interrupt();
            worker = null;
        }
        clearPendingTokenLocked();
    }

    private static void closeQuietly(BluetoothServerSocket socket) {
        if (socket == null) {
            return;
        }
        try {
            socket.close();
        } catch (IOException ignored) {
            // Best-effort cancellation.
        }
    }

    private static void closeQuietly(BluetoothSocket socket) {
        if (socket == null) {
            return;
        }
        try {
            socket.close();
        } catch (IOException ignored) {
            // Best-effort cancellation.
        }
    }

    private static void closeQuietly(ServerSocket socket) {
        if (socket == null) {
            return;
        }
        try {
            socket.close();
        } catch (IOException ignored) {
            // Best-effort cancellation.
        }
    }

    private static void closeQuietly(Socket socket) {
        if (socket == null) {
            return;
        }
        try {
            socket.close();
        } catch (IOException ignored) {
            // Best-effort cancellation.
        }
    }

    @Override
    public void close() {
        synchronized (lock) {
            if (destroyed) {
                return;
            }
            destroyed = true;
            generation++;
            closeResourcesLocked();
            permissionRequests.clear();
            activityRequests.clear();
            pendingAction = Action.NONE;
            pendingHostPort = 0;
            clearPendingTokenLocked();
        }
    }
}
