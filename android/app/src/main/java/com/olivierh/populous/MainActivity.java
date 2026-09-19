package com.olivierh.populous;

import android.app.Activity;
import android.content.pm.ApplicationInfo;
import android.os.Build;
import android.os.Bundle;
import android.os.Handler;
import android.os.Looper;
import android.util.Log;
import android.view.View;
import android.view.WindowInsets;
import android.view.WindowInsetsController;
import android.view.WindowManager;

import com.olivierh.populous.mobile.EbitenView;
import com.olivierh.populous.mobile.Mobile;

import go.Seq;

import java.io.File;

public final class MainActivity extends Activity {
    private EbitenView ebitenView;
    private BluetoothMultiplayer bluetoothMultiplayer;
    private final Handler bluetoothHandler = new Handler(Looper.getMainLooper());
    private boolean bluetoothPolling;
    private final Runnable bluetoothPoll = new Runnable() {
        @Override
        public void run() {
            if (!bluetoothPolling) {
                return;
            }
            try {
                // Drain a short burst without monopolising the Android UI thread.
                for (int index = 0; index < 8; index++) {
                    String command = Mobile.pollBluetoothCommand();
                    if (command == null || command.isEmpty()) {
                        break;
                    }
                    bluetoothMultiplayer.handleCommand(command);
                }
            } catch (Exception error) {
                Log.w("Populous", "Cannot process Bluetooth multiplayer command", error);
            }
            bluetoothHandler.postDelayed(this, 100L);
        }
    };

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);

        Seq.setContext(getApplicationContext());
        Mobile.setFilesDir(getFilesDir().getAbsolutePath());
        // Explicit, development-only capture: never microphone or system audio.
        if ((getApplicationInfo().flags & ApplicationInfo.FLAG_DEBUGGABLE) != 0) {
            String captureName = getIntent().getStringExtra("capture_audio");
            if (captureName != null && captureName.matches("[A-Za-z0-9_-]{1,80}\\.jsonl")) {
                try {
                    Mobile.setAudioCapturePath(new File(getFilesDir(), captureName).getAbsolutePath());
                } catch (Exception error) {
                    Log.w("Populous", "Cannot start source audio capture", error);
                }
            }
        }

        getWindow().addFlags(WindowManager.LayoutParams.FLAG_KEEP_SCREEN_ON);
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.P) {
            WindowManager.LayoutParams attributes = getWindow().getAttributes();
            attributes.layoutInDisplayCutoutMode =
                    WindowManager.LayoutParams.LAYOUT_IN_DISPLAY_CUTOUT_MODE_SHORT_EDGES;
            getWindow().setAttributes(attributes);
        }

        bluetoothMultiplayer = new BluetoothMultiplayer(this);
        Mobile.setBluetoothAvailable(bluetoothMultiplayer.isAvailable());

        ebitenView = new EbitenView(this);
        ebitenView.setFocusableInTouchMode(true);
        ebitenView.requestFocus();
        setContentView(ebitenView);
        bluetoothPolling = true;
        bluetoothHandler.post(bluetoothPoll);
        hideSystemUi();
    }

    @Override
    public void onRequestPermissionsResult(
            int requestCode, String[] permissions, int[] grantResults) {
        super.onRequestPermissionsResult(requestCode, permissions, grantResults);
        if (bluetoothMultiplayer != null) {
            bluetoothMultiplayer.onRequestPermissionsResult(requestCode);
        }
    }

    @Override
    protected void onActivityResult(int requestCode, int resultCode, android.content.Intent data) {
        super.onActivityResult(requestCode, resultCode, data);
        if (bluetoothMultiplayer != null) {
            bluetoothMultiplayer.onActivityResult(requestCode, resultCode);
        }
    }

    @Override
    protected void onPause() {
        Mobile.cancelInput();
        if (ebitenView != null) {
            ebitenView.suspendGame();
        }
        super.onPause();
    }

    @Override
    protected void onResume() {
        super.onResume();
        hideSystemUi();
        if (ebitenView != null) {
            ebitenView.resumeGame();
        }
    }

    @Override
    public void onWindowFocusChanged(boolean hasFocus) {
        super.onWindowFocusChanged(hasFocus);
        if (hasFocus) {
            hideSystemUi();
        }
    }

    @Override
    protected void onDestroy() {
        bluetoothPolling = false;
        bluetoothHandler.removeCallbacks(bluetoothPoll);
        // onPause can be a permission or pairing dialog; only final Activity
        // teardown cancels the corresponding Go multiplayer reservation.
        Mobile.cancelBluetooth();
        if (bluetoothMultiplayer != null) {
            bluetoothMultiplayer.close();
            bluetoothMultiplayer = null;
        }
        super.onDestroy();
    }

    private void hideSystemUi() {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.R) {
            getWindow().setDecorFitsSystemWindows(false);
            WindowInsetsController controller =
                    getWindow().getDecorView().getWindowInsetsController();
            if (controller != null) {
                controller.hide(WindowInsets.Type.statusBars() | WindowInsets.Type.navigationBars());
                controller.setSystemBarsBehavior(
                        WindowInsetsController.BEHAVIOR_SHOW_TRANSIENT_BARS_BY_SWIPE);
            }
            return;
        }

        getWindow().getDecorView().setSystemUiVisibility(
                View.SYSTEM_UI_FLAG_FULLSCREEN
                        | View.SYSTEM_UI_FLAG_HIDE_NAVIGATION
                        | View.SYSTEM_UI_FLAG_IMMERSIVE_STICKY
                        | View.SYSTEM_UI_FLAG_LAYOUT_FULLSCREEN
                        | View.SYSTEM_UI_FLAG_LAYOUT_HIDE_NAVIGATION
                        | View.SYSTEM_UI_FLAG_LAYOUT_STABLE);
    }
}
