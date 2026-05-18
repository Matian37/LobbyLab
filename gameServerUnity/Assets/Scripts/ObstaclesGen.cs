using System.Collections;
using System.Collections.Generic;
using UnityEngine;
using UnityEngine.SceneManagement;
using TMPro;
using System;

public class ObstaclesGen : MonoBehaviour
{
    public List<float> times = new List<float>();
    public List<float> speeds = new List<float>();
    int indx;
    public bool altLeft, altRight;

    public bool rival;
    public float timeBeforeMusic = 5;
    public float speed = 0.1f;
    float time = 0;
    public Transform player;
    public GameObject obs;
    public Vector3 leftWall, leftTor, rightTor, rightWall;
    public TextMeshProUGUI text, loseText;
    public GameObject winInfo, loseInfo;
    public string leftWinText, rightWinText;
    public int startGenerationCount;
    public float wyprzedzenie = 5;

    private void Awake()
    {
        Time.timeScale = 1;
    }

    private void Start()
    {
        Do();
    }

    void Do()
    {
        for (int i = 0; i < times.Count; i++)
        {
            if (times[i] < times[times.Count - 1] / 3) speeds.Add(speed);
            else if (times[i] < 2 * times[times.Count - 1] / 3) speeds.Add(speed * 1.35f);
            else speeds.Add(speed * 1.7f);
        }

        for (int i = 0; i < times.Count; i++)
        {
            times[i] += timeBeforeMusic;
            //if (i == 0) Debug.Log(times[i]);
            generate(times[i], speeds[i], i);
        }
    }

    private void Update()
    {
        time += Time.deltaTime;
        if (altLeft) return;

        if(time >= timeBeforeMusic)
            text.text = Mathf.RoundToInt((time - timeBeforeMusic) / (times[times.Count - 1] + 2) * 100).ToString() + "%";
    }

    int lastGenerated = 0;

    void generate(float tt, float sp, int j)
    {
        float z = player.position.z - sp * tt;
        int x;
        List<int> l = new List<int>();
        if (!altLeft && !altRight)
        {
            l = new List<int>{1, 2, 3, 4};

            if (lastGenerated == 1)
            {
                l.Remove(3);
                l.Remove(1);
            }
            else if (lastGenerated == 2)
            {
                l.Remove(4);
                l.Remove(2);
            }
            else if (lastGenerated == 3)
            {
                l.Remove(4);
                l.Remove(2);
            }
            else if(lastGenerated == 4)
            {
                l.Remove(3);
                l.Remove(1);
            }
            x = l[UnityEngine.Random.Range(0, l.Count)];
        }
        else if (altLeft)
        {
            l = new List<int> { 1, 2, 3 };

            if (lastGenerated == 1)
            {
                l.Remove(1);
                l.Remove(3);
            }
            else if (lastGenerated == 2)
            {
                l.Remove(2);
            }
            else if (lastGenerated == 3)
            {
                l.Remove(2);
            }
            x = l[UnityEngine.Random.Range(0, l.Count)];
        }
        else
        {
            l = new List<int> { 1, 2, 4 };
            if (lastGenerated == 1)
            {
                l.Remove(1);
            }
            else if (lastGenerated == 2)
            {
                l.Remove(2);
                l.Remove(4);
            }
            else if (lastGenerated == 4)
            {
                l.Remove(1);
            }
            x = l[UnityEngine.Random.Range(0, l.Count)];
        }
        lastGenerated = x;
        Vector3 pos = new Vector3();
        Quaternion rot = Quaternion.Euler(0, 0, 0);
        switch (x)
        {
            case 1:
                pos = rightTor;
                break;
            case 2:
                pos = leftTor;
                break;
            case 3:
                pos = leftWall;
                rot = Quaternion.Euler(0, 0, 90);
                break;
            case 4:
                pos = rightWall;
                rot = Quaternion.Euler(0, 0, 270);
                break;
            default:
                break;

        }
        pos.z = z;
        Obstacle o = Instantiate(obs, pos, rot).GetComponent<Obstacle>();
        o.GetComponent<ElementControl>().SetIndx(j, this);
        if (altLeft) o.isLeft = true;
        else o.isLeft = false;
        if (x > 2) o.isWall = true;
        if (x == 3) o.left = true;
        else o.left = false;
    }
}
