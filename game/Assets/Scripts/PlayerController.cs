using System.Collections;
using System.Collections.Generic;
using UnityEngine;

public class PlayerController : MonoBehaviour
{
    int akt = 0; //0, 1
    Vector3 startPosition, desiredPos;
    bool isMoving, wall, wall2;
    public float speed = 0.01f;
    float c = 0;
    public bool altLeft, altRight;
    public Vector3 leftWall, leftTor, rightTor, rightWall;
    public ObstaclesGen gen;
    public GameObject escape;
    [HideInInspector] public bool touchedWall = false;
    public LivesManager livesManager;
    
    private void Update()
    {
        if (escape.activeInHierarchy) return;

        if (altRight)
        {
            if (Input.GetKeyDown(SceneTransport.Instance.binds[2]))
                move(0);
            if (Input.GetKeyDown(SceneTransport.Instance.binds[3]))
                move(1);
        }
        else
        {
            if (Input.GetKeyDown(SceneTransport.Instance.binds[0]))
                move(0);
            if (Input.GetKeyDown(SceneTransport.Instance.binds[1]))
                move(1);
        }
        if (isMoving)
        {
            transform.position = Vector3.Lerp(startPosition, desiredPos, c);
            c += speed * Time.deltaTime;
            if(c >= 1)
            {
                isMoving = false;
                if (wall)
                {
                    wall = false;
                    isMoving = true;

                    Vector3 pom = startPosition;
                    startPosition = desiredPos;
                    desiredPos = pom;

                    c = 0;
                    Invoke("Check", 0.03f);
                    return;
                }
            }
        }
    }

    void Check()
    {
        //Debug.Log(touchedWall);
        if (!touchedWall)
            livesManager.EraseLife();
        touchedWall = false;
    }

    void move(int x)
    {
        if(x == 0)
        {
            if(akt == 1) //go lewy tor
            {
                akt = 0;
                desiredPos = leftTor;
                wall = false;
            }
            else //lewa sciana
            {
                if (altRight || isMoving) return;
                wall = true;
                desiredPos = leftWall;
            }
        }
        else
        {
            if(akt == 0) //go prawy tor
            {
                akt = 1;
                wall = false;
                desiredPos = rightTor;
            }
            else //prawa sciana
            {
                if (altLeft || isMoving) return;
                wall = true;
                desiredPos = rightWall;
            }
        }
    
        startPosition = transform.position;
        isMoving = true;
        c = 0;
    }
}
