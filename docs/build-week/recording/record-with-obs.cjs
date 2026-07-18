const fs = require("fs");
const os = require("os");
const path = require("path");
const OBSWebSocket = require("obs-websocket-js").default;

const repoRoot = path.resolve(__dirname, "..", "..", "..");
const outputDir = path.join(repoRoot, "out", "build-week-video");
const presentationPath = path.join(outputDir, "presentation.html");
const narrationPath = path.join(outputDir, "narration-neural-fast.mp3");
const configPath = path.join(
  process.env.APPDATA || path.join(os.homedir(), "AppData", "Roaming"),
  "obs-studio",
  "plugin_config",
  "obs-websocket",
  "config.json",
);
const durationSeconds = Number(process.argv[2] || "164.5");

function requireFile(filePath) {
  if (!fs.existsSync(filePath)) {
    throw new Error(`Required recording input is missing: ${filePath}`);
  }
}

async function call(obs, requestType, requestData = undefined) {
  return obs.call(requestType, requestData);
}

async function ensureProfile(obs, profileName) {
  const profiles = await call(obs, "GetProfileList");
  if (!profiles.profiles.some((profile) => (profile.profileName || profile) === profileName)) {
    await call(obs, "CreateProfile", { profileName });
  }
  if (profiles.currentProfileName !== profileName) {
    await call(obs, "SetCurrentProfile", { profileName });
  }

  const parameters = [
    ["Video", "BaseCX", "1920"],
    ["Video", "BaseCY", "1080"],
    ["Video", "OutputCX", "1920"],
    ["Video", "OutputCY", "1080"],
    ["Video", "FPSType", "0"],
    ["Video", "FPSCommon", "30"],
    ["SimpleOutput", "FilePath", outputDir],
    ["SimpleOutput", "RecFormat2", "hybrid_mp4"],
  ];
  for (const [parameterCategory, parameterName, parameterValue] of parameters) {
    await call(obs, "SetProfileParameter", {
      parameterCategory,
      parameterName,
      parameterValue,
    });
  }
}

async function ensureSceneCollection(obs, collectionName) {
  const collections = await call(obs, "GetSceneCollectionList");
  if (!collections.sceneCollections.includes(collectionName)) {
    await call(obs, "CreateSceneCollection", { sceneCollectionName: collectionName });
  } else if (collections.currentSceneCollectionName !== collectionName) {
    await call(obs, "SetCurrentSceneCollection", { sceneCollectionName: collectionName });
  }
}

async function ensureScene(obs, sceneName) {
  const scenes = await call(obs, "GetSceneList");
  if (!scenes.scenes.some((scene) => scene.sceneName === sceneName)) {
    await call(obs, "CreateScene", { sceneName });
  }
}

async function ensureInput(obs, sceneName, inputName, inputKind, inputSettings) {
  const inputs = await call(obs, "GetInputList");
  if (!inputs.inputs.some((input) => input.inputName === inputName)) {
    await call(obs, "CreateInput", {
      sceneName,
      inputName,
      inputKind,
      inputSettings,
      sceneItemEnabled: true,
    });
    return;
  }
  await call(obs, "SetInputSettings", {
    inputName,
    inputSettings,
    overlay: false,
  });
}

async function run() {
  requireFile(presentationPath);
  requireFile(narrationPath);
  requireFile(configPath);
  fs.mkdirSync(outputDir, { recursive: true });

  const config = JSON.parse(fs.readFileSync(configPath, "utf8"));
  const obs = new OBSWebSocket();
  const url = `ws://127.0.0.1:${config.server_port || 4455}`;
  await obs.connect(url, config.server_password);

  let recordingStarted = false;
  try {
    const version = await call(obs, "GetVersion");
    console.log(`Connected to OBS ${version.obsVersion}.`);
    console.log("Configuring isolated BuildWeek profile.");
    await ensureProfile(obs, "BuildWeek");
    console.log("Configuring isolated BuildWeek scene collection.");
    await ensureSceneCollection(obs, "BuildWeek");
    await ensureScene(obs, "Holding");
    await ensureScene(obs, "Build Week Presentation");

    await ensureInput(obs, "Build Week Presentation", "Build Week Slides", "browser_source", {
      is_local_file: true,
      local_file: presentationPath.replaceAll("\\", "/"),
      width: 1920,
      height: 1080,
      shutdown: true,
      restart_when_active: true,
      reroute_audio: false,
    });
    await ensureInput(obs, "Build Week Presentation", "Build Week Narration", "ffmpeg_source", {
      is_local_file: true,
      local_file: narrationPath.replaceAll("\\", "/"),
      looping: false,
      restart_on_activate: true,
      close_when_inactive: true,
    });

    await call(obs, "SetCurrentProgramScene", { sceneName: "Holding" });
    const status = await call(obs, "GetRecordStatus");
    if (status.outputActive) {
      throw new Error("OBS is already recording; refusing to interfere with it.");
    }

    console.log(`Starting ${durationSeconds.toFixed(1)} second recording.`);
    await call(obs, "StartRecord");
    recordingStarted = true;
    await call(obs, "SetCurrentProgramScene", { sceneName: "Build Week Presentation" });
    await call(obs, "PressInputPropertiesButton", {
      inputName: "Build Week Slides",
      propertyName: "refreshnocache",
    });

    const startedAt = Date.now();
    while ((Date.now() - startedAt) / 1000 < durationSeconds) {
      const elapsed = (Date.now() - startedAt) / 1000;
      const remaining = Math.max(0, durationSeconds - elapsed);
      console.log(`Recording: ${elapsed.toFixed(0)}s elapsed, ${remaining.toFixed(0)}s remaining.`);
      await new Promise((resolve) => setTimeout(resolve, Math.min(20000, remaining * 1000)));
    }

    await call(obs, "SaveSourceScreenshot", {
      sourceName: "Build Week Slides",
      imageFormat: "png",
      imageFilePath: path.join(outputDir, "thumbnail.png"),
      imageWidth: 1920,
      imageHeight: 1080,
      imageCompressionQuality: -1,
    });

    const stopped = await call(obs, "StopRecord");
    recordingStarted = false;
    console.log(`Recording complete: ${stopped.outputPath}`);
  } finally {
    if (recordingStarted) {
      try {
        await call(obs, "StopRecord");
      } catch {
        // Preserve the original failure while still attempting a clean stop.
      }
    }
    try {
      await obs.disconnect();
    } catch {
      // OBS may close the socket while switching profiles or collections.
    }
  }
}

run().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
